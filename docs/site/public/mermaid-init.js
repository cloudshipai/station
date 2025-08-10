// Initialize Mermaid diagrams
import mermaid from 'https://cdn.skypack.dev/mermaid@latest';

mermaid.initialize({
  startOnLoad: true,
  theme: 'default',
  securityLevel: 'loose',
  themeVariables: {
    primaryColor: '#667eea',
    primaryTextColor: '#fff',
    primaryBorderColor: '#764ba2',
    lineColor: '#f093fb',
    sectionBkgColor: '#a8e6cf',
    altSectionBkgColor: '#ffd93d'
  }
});

// Convert mermaid code blocks to rendered diagrams
document.addEventListener('DOMContentLoaded', function() {
  // Find all code blocks with language "mermaid"
  const mermaidBlocks = document.querySelectorAll('code.language-mermaid');
  
  mermaidBlocks.forEach((block, index) => {
    const diagramCode = block.textContent;
    const diagramId = `mermaid-diagram-${index}`;
    
    // Create a div to render the diagram
    const diagramDiv = document.createElement('div');
    diagramDiv.id = diagramId;
    diagramDiv.className = 'mermaid';
    diagramDiv.textContent = diagramCode;
    
    // Replace the code block with the diagram
    block.parentElement.replaceWith(diagramDiv);
    
    // Render the diagram
    mermaid.init(undefined, diagramDiv);
  });
});