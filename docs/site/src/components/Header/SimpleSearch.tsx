/** @jsxImportSource react */
import { useState, useCallback, useRef, useEffect } from 'react'

export default function SimpleSearch() {
  const [isOpen, setIsOpen] = useState(false)
  const [searchResults, setSearchResults] = useState<Array<{title: string, url: string, content: string}>>([])
  const [query, setQuery] = useState('')
  const searchRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Sample search data - in a real app this would come from a search index
  const searchData = [
    { title: 'Introduction', url: '/station/en/introduction/', content: 'Station is a lightweight runtime for deployable sub-agents' },
    { title: 'MCP Quick Start', url: '/station/en/mcp-quickstart/', content: 'Get started with MCP integration' },
    { title: 'Installation', url: '/station/en/installation/', content: 'Install Station on your system' },
    { title: 'Creating Agents', url: '/station/en/agents/creating/', content: 'Learn how to create and configure agents' },
    { title: 'Claude Desktop Setup', url: '/station/en/mcp/claude-desktop/', content: 'Configure Station with Claude Desktop' },
    { title: 'MCP Tools & Commands', url: '/station/en/mcp/tools/', content: 'Available MCP tools and commands' }
  ]

  const handleSearch = useCallback((searchTerm: string) => {
    if (!searchTerm.trim()) {
      setSearchResults([])
      return
    }

    const results = searchData.filter(item => 
      item.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
      item.content.toLowerCase().includes(searchTerm.toLowerCase())
    ).slice(0, 5)
    
    setSearchResults(results)
  }, [])

  const onOpen = useCallback(() => {
    setIsOpen(true)
    setTimeout(() => inputRef.current?.focus(), 100)
  }, [])

  const onClose = useCallback(() => {
    setIsOpen(false)
    setQuery('')
    setSearchResults([])
  }, [])

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === '/' && !isOpen) {
        e.preventDefault()
        onOpen()
      }
      if (e.key === 'Escape' && isOpen) {
        onClose()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onOpen, onClose])

  // Handle clicks outside search
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
        onClose()
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isOpen, onClose])

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value
    setQuery(value)
    handleSearch(value)
  }

  return (
    <div className="simple-search" ref={searchRef}>
      <button
        type="button"
        onClick={onOpen}
        className="search-input"
      >
        <svg width="20" height="20" fill="none" viewBox="0 0 24 24">
          <path
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        <span>Search docs</span>
        <span className="search-hint">
          <kbd>/</kbd>
        </span>
      </button>

      {isOpen && (
        <div className="search-modal">
          <div className="search-modal-content">
            <div className="search-input-container">
              <svg width="20" height="20" fill="none" viewBox="0 0 24 24" className="search-icon">
                <path
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={handleInputChange}
                placeholder="Search documentation..."
                className="search-modal-input"
              />
              <button onClick={onClose} className="search-close">
                ✕
              </button>
            </div>
            
            <div className="search-results">
              {searchResults.length > 0 ? (
                <ul>
                  {searchResults.map((result, index) => (
                    <li key={index}>
                      <a href={result.url} onClick={onClose}>
                        <div className="result-title">{result.title}</div>
                        <div className="result-content">{result.content}</div>
                      </a>
                    </li>
                  ))}
                </ul>
              ) : query.trim() ? (
                <div className="no-results">No results found for "{query}"</div>
              ) : (
                <div className="search-tips">
                  <div className="tip">💡 Start typing to search documentation</div>
                  <div className="tip">⌘K or / to open search</div>
                  <div className="tip">↑↓ to navigate results</div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}