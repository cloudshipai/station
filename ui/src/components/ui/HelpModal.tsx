import React, { useState, useEffect, useRef } from 'react';
import { X, Info } from 'lucide-react';

interface TocItem {
  id: string;
  label: string;
}

interface HelpModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  pageDescription?: string;
  tocItems?: TocItem[];
  children: React.ReactNode;
}

export const HelpModal: React.FC<HelpModalProps> = ({ isOpen, onClose, title, pageDescription, tocItems, children }) => {
  const [activeSection, setActiveSection] = useState<string | null>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen || !tocItems || tocItems.length === 0) return;

    const contentElement = contentRef.current;
    if (!contentElement) return;

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id);
          }
        });
      },
      {
        root: contentElement,
        rootMargin: '-20% 0px -70% 0px',
        threshold: 0,
      }
    );

    tocItems.forEach((item) => {
      const element = document.getElementById(item.id);
      if (element) {
        observer.observe(element);
      }
    });

    return () => observer.disconnect();
  }, [isOpen, tocItems]);

  const scrollToSection = (sectionId: string) => {
    const element = document.getElementById(sectionId);
    if (element && contentRef.current) {
      const offsetTop = element.offsetTop - contentRef.current.offsetTop;
      contentRef.current.scrollTo({
        top: offsetTop - 20,
        behavior: 'smooth',
      });
    }
  };

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-lg shadow-2xl max-w-4xl w-full max-h-[85vh] overflow-hidden flex flex-col border border-gray-200"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="border-b border-gray-200 bg-white">
          {/* Title row */}
          <div className="flex items-center justify-between px-6 py-5">
            <h2 className="text-xl font-semibold text-gray-900">{title}</h2>
            <button
              onClick={onClose}
              className="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          {/* TOC Navigation */}
          {tocItems && tocItems.length > 0 && (
            <div className="px-6 pb-4 border-t border-gray-100">
              <nav className="flex gap-2 overflow-x-auto py-3 scrollbar-thin">
                {tocItems.map((item) => (
                  <button
                    key={item.id}
                    onClick={() => scrollToSection(item.id)}
                    className={`
                      px-3 py-1.5 rounded-lg text-sm whitespace-nowrap transition-all font-medium
                      ${
                        activeSection === item.id
                          ? 'bg-[#0084FF] text-white shadow-sm'
                          : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
                      }
                    `}
                  >
                    {item.label}
                  </button>
                ))}
              </nav>
            </div>
          )}
        </div>

        {/* Content */}
        <div ref={contentRef} className="flex-1 overflow-y-auto p-6">
          {pageDescription && (
            <div className="mb-8">
              <div className="flex items-center gap-2 mb-3">
                <Info className="h-5 w-5 text-[#0084FF]" />
                <div className="text-xs font-bold text-gray-900 uppercase tracking-wider">About This Page</div>
              </div>
              <div className="bg-[#F8FAFB] border border-gray-200 rounded-lg p-5">
                <div className="text-sm text-gray-700 leading-relaxed">{pageDescription}</div>
              </div>
            </div>
          )}
          {children}
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-gray-200 bg-white">
          <button
            onClick={onClose}
            className="px-5 py-2.5 bg-gray-900 hover:bg-gray-800 text-white rounded-lg text-sm font-medium transition-colors shadow-sm"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
