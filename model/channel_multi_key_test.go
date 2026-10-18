package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMultiKeyChannel(mode constant.MultiKeyMode, statusList map[int]int) *Channel {
	return &Channel{
		Id:   9001,
		Key:  "key-a\nkey-b\nkey-c",
		Name: "multi-key-selection",
		ChannelInfo: ChannelInfo{
			IsMultiKey:         true,
			MultiKeySize:       3,
			MultiKeyMode:       mode,
			MultiKeyStatusList: statusList,
		},
	}
}

// Sticky mode is the only multi-key strategy that must return the SAME key for
// consecutive requests; it may only advance when the current key gets disabled,
// and must fall back to an earlier key once that key is enabled again.
func TestGetNextEnabledKeyStickyStaysOnFirstAvailableKey(t *testing.T) {
	testCases := []struct {
		name        string
		statusList  map[int]int
		expectedKey string
		expectedIdx int
	}{
		{
			name:        "all keys enabled uses the first key",
			statusList:  nil,
			expectedKey: "key-a",
			expectedIdx: 0,
		},
		{
			name:        "first key auto disabled falls through to the second",
			statusList:  map[int]int{0: common.ChannelStatusAutoDisabled},
			expectedKey: "key-b",
			expectedIdx: 1,
		},
		{
			name: "first two keys disabled falls through to the third",
			statusList: map[int]int{
				0: common.ChannelStatusAutoDisabled,
				1: common.ChannelStatusManuallyDisabled,
			},
			expectedKey: "key-c",
			expectedIdx: 2,
		},
		{
			name:        "later key disabled does not move off the first key",
			statusList:  map[int]int{1: common.ChannelStatusAutoDisabled},
			expectedKey: "key-a",
			expectedIdx: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			channel := newMultiKeyChannel(constant.MultiKeyModeSticky, tc.statusList)
			// Repeat to prove the selection is sticky rather than rotating per request.
			for i := 0; i < 5; i++ {
				key, idx, apiErr := channel.GetNextEnabledKey(nil)
				require.Nil(t, apiErr)
				assert.Equal(t, tc.expectedKey, key)
				assert.Equal(t, tc.expectedIdx, idx)
			}
		})
	}

	t.Run("re-enabling an earlier key moves traffic back to it", func(t *testing.T) {
		channel := newMultiKeyChannel(constant.MultiKeyModeSticky, map[int]int{0: common.ChannelStatusAutoDisabled})
		key, idx, apiErr := channel.GetNextEnabledKey(nil)
		require.Nil(t, apiErr)
		require.Equal(t, "key-b", key)
		require.Equal(t, 1, idx)

		delete(channel.ChannelInfo.MultiKeyStatusList, 0)
		key, idx, apiErr = channel.GetNextEnabledKey(nil)
		require.Nil(t, apiErr)
		assert.Equal(t, "key-a", key)
		assert.Equal(t, 0, idx)
	})

	t.Run("no enabled key returns an error instead of a disabled key", func(t *testing.T) {
		channel := newMultiKeyChannel(constant.MultiKeyModeSticky, map[int]int{
			0: common.ChannelStatusAutoDisabled,
			1: common.ChannelStatusAutoDisabled,
			2: common.ChannelStatusManuallyDisabled,
		})
		key, _, apiErr := channel.GetNextEnabledKey(nil)
		require.NotNil(t, apiErr)
		assert.Empty(t, key)
	})
}

// A retry must land on a key the request has not used yet, otherwise a multi-key
// channel keeps replaying the key that just failed. Once every enabled key has been
// tried the exclusion is dropped so the caller still gets the upstream error rather
// than a "no available key" routing error.
func TestGetNextEnabledKeySkipsKeysAlreadyTried(t *testing.T) {
	testCases := []struct {
		name        string
		mode        constant.MultiKeyMode
		statusList  map[int]int
		tried       map[int]struct{}
		expectedKey string
		expectedIdx int
	}{
		{
			name:        "sticky moves to the next key after the first was tried",
			mode:        constant.MultiKeyModeSticky,
			tried:       map[int]struct{}{0: {}},
			expectedKey: "key-b",
			expectedIdx: 1,
		},
		{
			name:        "sticky skips tried keys and disabled keys together",
			mode:        constant.MultiKeyModeSticky,
			statusList:  map[int]int{1: common.ChannelStatusAutoDisabled},
			tried:       map[int]struct{}{0: {}},
			expectedKey: "key-c",
			expectedIdx: 2,
		},
		{
			name:        "random has a single candidate left when the others were tried",
			mode:        constant.MultiKeyModeRandom,
			tried:       map[int]struct{}{0: {}, 1: {}},
			expectedKey: "key-c",
			expectedIdx: 2,
		},
		{
			name:        "unknown mode also honours the exclusion",
			mode:        constant.MultiKeyMode("weighted"),
			tried:       map[int]struct{}{0: {}},
			expectedKey: "key-b",
			expectedIdx: 1,
		},
		{
			name:        "all keys tried falls back to the first enabled key",
			mode:        constant.MultiKeyModeSticky,
			tried:       map[int]struct{}{0: {}, 1: {}, 2: {}},
			expectedKey: "key-a",
			expectedIdx: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			channel := newMultiKeyChannel(tc.mode, tc.statusList)
			key, idx, apiErr := channel.GetNextEnabledKey(tc.tried)
			require.Nil(t, apiErr)
			assert.Equal(t, tc.expectedKey, key)
			assert.Equal(t, tc.expectedIdx, idx)
		})
	}
}

// Polling keeps its persisted cursor, but must still skip keys the current request
// already spent.
func TestGetNextEnabledKeyPollingSkipsTriedKeys(t *testing.T) {
	setupChannelStatusTest(t)

	channel := newMultiKeyChannel(constant.MultiKeyModePolling, nil)
	channel.Status = common.ChannelStatusEnabled
	require.NoError(t, DB.Create(channel).Error)

	key, idx, apiErr := channel.GetNextEnabledKey(nil)
	require.Nil(t, apiErr)
	require.Equal(t, "key-a", key)
	require.Equal(t, 0, idx)
	require.Equal(t, 1, channel.ChannelInfo.MultiKeyPollingIndex)

	// The cursor points at key-b, which this request already used, so polling has to
	// move on to key-c instead of handing back the failed key.
	key, idx, apiErr = channel.GetNextEnabledKey(map[int]struct{}{0: {}, 1: {}})
	require.Nil(t, apiErr)
	assert.Equal(t, "key-c", key)
	assert.Equal(t, 2, idx)
}

// The relay retry loop uses HasUntriedEnabledKey to decide whether another attempt
// on the same channel can still reach a fresh key.
func TestHasUntriedEnabledKey(t *testing.T) {
	testCases := []struct {
		name       string
		statusList map[int]int
		tried      map[int]struct{}
		expected   bool
	}{
		{
			name:     "nothing tried yet",
			tried:    nil,
			expected: true,
		},
		{
			name:     "some keys still untried",
			tried:    map[int]struct{}{0: {}, 1: {}},
			expected: true,
		},
		{
			name:     "every key tried",
			tried:    map[int]struct{}{0: {}, 1: {}, 2: {}},
			expected: false,
		},
		{
			name:       "remaining untried keys are all disabled",
			statusList: map[int]int{1: common.ChannelStatusAutoDisabled, 2: common.ChannelStatusManuallyDisabled},
			tried:      map[int]struct{}{0: {}},
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			channel := newMultiKeyChannel(constant.MultiKeyModeSticky, tc.statusList)
			assert.Equal(t, tc.expected, channel.HasUntriedEnabledKey(tc.tried))
		})
	}

	t.Run("single key channels never rotate", func(t *testing.T) {
		channel := &Channel{Id: 9002, Key: "only-key"}
		assert.False(t, channel.HasUntriedEnabledKey(nil))
	})
}

// Sticky mode must switch keys for the whole site, not just for the failing request:
// once a key burns through the channel's retry budget it is retired, so every later
// request starts from the next key even after a restart.
func TestRecordMultiKeyFailureRetiresKeyAfterRetryTimes(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:   "sticky-retire",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:         true,
			MultiKeySize:       2,
			MultiKeyMode:       constant.MultiKeyModeSticky,
			MultiKeyRetryTimes: 2,
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	// The first failure only counts; the key stays in service.
	require.False(t, RecordMultiKeyFailure(channel.Id, 0, "upstream rate limited"))
	stored, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, 1, stored.ChannelInfo.MultiKeyFailureCount[0])
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	_, idx, apiErr := stored.GetNextEnabledKey(nil)
	require.Nil(t, apiErr)
	assert.Equal(t, 0, idx)

	// The second failure uses up the budget: the key is retired and the sticky cursor
	// moves on for everyone.
	require.True(t, RecordMultiKeyFailure(channel.Id, 0, "upstream rate limited"))
	stored, err = GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.ChannelInfo.MultiKeyStatusList[0])
	assert.Equal(t, "upstream rate limited", stored.ChannelInfo.MultiKeyDisabledReason[0])
	key, idx, apiErr := stored.GetNextEnabledKey(nil)
	require.Nil(t, apiErr)
	assert.Equal(t, "key-b", key)
	assert.Equal(t, 1, idx)

	// Re-enabling the channel through the usual status path brings the key back as the
	// sticky key with a clean streak.
	require.False(t, RecordMultiKeyFailure(channel.Id, 1, "upstream rate limited"))
	require.True(t, RecordMultiKeyFailure(channel.Id, 1, "upstream rate limited"))
	stored, err = GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusAutoDisabled, stored.Status, "a channel with no usable key is disabled")
	_, _, apiErr = stored.GetNextEnabledKey(nil)
	require.NotNil(t, apiErr, "no key left to serve requests")

	require.True(t, UpdateChannelStatus(channel.Id, "key-a", common.ChannelStatusEnabled, ""))
	stored, err = GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Zero(t, stored.ChannelInfo.MultiKeyFailureCount[0])
	key, idx, apiErr = stored.GetNextEnabledKey(nil)
	require.Nil(t, apiErr)
	assert.Equal(t, "key-a", key)
	assert.Equal(t, 0, idx)
}

// A successful call clears the streak so unrelated one-off errors never add up to a
// switch, and non-sticky strategies never retire keys at all because they already
// spread traffic across the pool on every request.
func TestMultiKeyFailureStreakResetAndModeScope(t *testing.T) {
	setupChannelStatusTest(t)

	stickyChannel := Channel{
		Name:   "sticky-reset",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:         true,
			MultiKeySize:       2,
			MultiKeyMode:       constant.MultiKeyModeSticky,
			MultiKeyRetryTimes: 3,
		},
	}
	require.NoError(t, DB.Create(&stickyChannel).Error)

	require.False(t, RecordMultiKeyFailure(stickyChannel.Id, 0, "boom"))
	require.False(t, RecordMultiKeyFailure(stickyChannel.Id, 0, "boom"))
	stored, err := GetChannelById(stickyChannel.Id, true)
	require.NoError(t, err)
	require.Equal(t, 2, stored.ChannelInfo.MultiKeyFailureCount[0])

	ResetMultiKeyFailure(stickyChannel.Id, 0)
	stored, err = GetChannelById(stickyChannel.Id, true)
	require.NoError(t, err)
	assert.Zero(t, stored.ChannelInfo.MultiKeyFailureCount[0])
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)

	pollingChannel := Channel{
		Name:   "polling-no-retire",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:         true,
			MultiKeySize:       2,
			MultiKeyMode:       constant.MultiKeyModePolling,
			MultiKeyRetryTimes: 1,
		},
	}
	require.NoError(t, DB.Create(&pollingChannel).Error)

	assert.False(t, RecordMultiKeyFailure(pollingChannel.Id, 0, "boom"))
	stored, err = GetChannelById(pollingChannel.Id, true)
	require.NoError(t, err)
	assert.Empty(t, stored.ChannelInfo.MultiKeyFailureCount)
	assert.Empty(t, stored.ChannelInfo.MultiKeyStatusList)
}

// The per-channel budget falls back to the shared default when it is left unset.
func TestGetMultiKeyRetryTimes(t *testing.T) {
	channel := newMultiKeyChannel(constant.MultiKeyModeSticky, nil)
	assert.Equal(t, DefaultMultiKeyRetryTimes, channel.GetMultiKeyRetryTimes())

	channel.ChannelInfo.MultiKeyRetryTimes = 7
	assert.Equal(t, 7, channel.GetMultiKeyRetryTimes())
}
