import type { MarkdownHeading } from 'astro'
import type { FunctionalComponent } from 'preact'
import { unescape } from 'html-escaper'
import { useState, useEffect, useRef } from 'preact/hooks'

type ItemOffsets = {
  id: string
  topOffset: number
}

const TableOfContents: FunctionalComponent<{ headings: MarkdownHeading[] }> = ({
  headings = []
}) => {
  const toc = useRef<HTMLUListElement>()
  const onThisPageID = 'on-this-page-heading'
  const itemOffsets = useRef<ItemOffsets[]>([])
  const [currentID, setCurrentID] = useState('overview')
  useEffect(() => {
    const getItemOffsets = () => {
      const titles = document.querySelectorAll('article :is(h1, h2, h3, h4)')
      itemOffsets.current = Array.from(titles).map((title) => ({
        id: title.id,
        topOffset: title.getBoundingClientRect().top + window.scrollY
      }))
    }

    getItemOffsets()
    window.addEventListener('resize', getItemOffsets)

    return () => {
      window.removeEventListener('resize', getItemOffsets)
    }
  }, [])

  useEffect(() => {
    if (!toc.current) return

    const setCurrent: IntersectionObserverCallback = (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          const { id } = entry.target
          if (id === onThisPageID) continue
          setCurrentID(entry.target.id)
          break
        }
      }
    }

    const observerOptions: IntersectionObserverInit = {
      // Negative top margin accounts for `scroll-margin`.
      // Negative bottom margin means heading needs to be towards top of viewport to trigger intersection.
      rootMargin: '-100px 0% -66%',
      threshold: 1
    }

    const headingsObserver = new IntersectionObserver(
      setCurrent,
      observerOptions
    )

    // Observe all the headings in the main page content.
    document
      .querySelectorAll('article :is(h1,h2,h3)')
      .forEach((h) => headingsObserver.observe(h))

    // Stop observing when the component is unmounted.
    return () => headingsObserver.disconnect()
  }, [toc.current])

  const onLinkClick = (e) => {
    setCurrentID(e.target.getAttribute('href').replace('#', ''))
  }

  return (
    <div className="modern-toc">
      <div className="toc-header">
        <svg className="toc-header-icon" viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
          <path d="M3 4h18v2H3V4zm0 7h18v2H3v-2zm0 7h18v2H3v-2z"/>
        </svg>
        <h2 id={onThisPageID} className="toc-heading">
          On this page
        </h2>
      </div>
      
      {headings.filter(({ depth }) => depth > 1 && depth < 4).length > 0 ? (
        <div className="toc-content">
          <ul ref={toc} className="toc-list">
            {headings
              .filter(({ depth }) => depth > 1 && depth < 4)
              .map((heading) => (
                <li
                  key={heading.slug}
                  className={`toc-item depth-${heading.depth} ${
                    currentID === heading.slug ? 'toc-item-active' : ''
                  }`.trim()}
                >
                  <a 
                    href={`#${heading.slug}`} 
                    className="toc-link"
                    onClick={onLinkClick}
                  >
                    <span className="toc-link-text">{unescape(heading.text)}</span>
                  </a>
                </li>
              ))}
          </ul>
        </div>
      ) : (
        <div className="toc-empty">
          <p className="toc-empty-text">No headings found</p>
        </div>
      )}
    </div>
  )
}

export default TableOfContents
