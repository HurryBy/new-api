/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useRef, type CSSProperties } from 'react'

// Fixed positions keep the atmosphere stable across renders and language changes.
const particles = Array.from({ length: 48 }, (_, index) => ({
  id: `particle-${index}`,
  style: {
    '--particle-x': `${(index * 37 + 7) % 100}%`,
    '--particle-y': `${(index * 23 + 3) % 100}%`,
    '--particle-size': `${index % 3 === 0 ? 3 : 2}px`,
    '--particle-delay': `${-(index % 17)}s`,
    '--particle-duration': `${18 + (index % 7) * 3}s`,
  } as CSSProperties,
}))

export function AstralAtmosphere() {
  const atmosphere = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const updateVisibility = () => {
      if (atmosphere.current) {
        atmosphere.current.dataset.paused = String(document.hidden)
      }
    }
    updateVisibility()
    document.addEventListener('visibilitychange', updateVisibility)
    return () =>
      document.removeEventListener('visibilitychange', updateVisibility)
  }, [])

  return (
    <div ref={atmosphere} className='astral-atmosphere' aria-hidden='true'>
      <div className='astral-nebula astral-nebula-violet' />
      <div className='astral-nebula astral-nebula-amber' />
      <div className='astral-nebula astral-nebula-bottom' />
      <div className='astral-atmosphere-grid' />
      <div className='astral-orbital-lines' />
      {particles.map((particle) => (
        <i
          key={particle.id}
          className='astral-particle'
          style={particle.style}
        />
      ))}
    </div>
  )
}
