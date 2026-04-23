import React from 'react';
import { FaGithub, FaBook, FaTerminal } from 'react-icons/fa';

const Navbar = () => {
  return (
    <nav className="sticky top-0 z-40 w-full glass-panel border-b border-border-dark">
      <div className="max-w-7xl mx-auto px-6 h-20 flex items-center justify-between">
        <div className="flex items-center gap-3 group cursor-pointer">
          <div className="size-8 bg-primary/20 rounded-md flex items-center justify-center border border-primary/40 group-hover:bg-primary group-hover:border-primary transition-all duration-300">
            <FaTerminal className="text-primary group-hover:text-background-dark text-xl transition-colors" />
          </div>
          <span className="text-xl font-bold tracking-tight">
            Koded<span className="text-primary">_</span>
          </span>
        </div>
        
        <div className="flex items-center gap-4">
          <a 
            href="#documentation"
            className="hidden sm:flex items-center gap-2 text-sm font-medium text-text-muted hover:text-white transition-colors"
          >
            <FaBook className="w-4 h-4" />
            <span>Documentation</span>
          </a>
          <a 
            href="https://github.com/Koded0214h/koded-cli"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-4 py-2 bg-surface-dark border border-border-dark rounded-lg text-sm font-bold text-white hover:border-primary/50 hover:shadow-[0_0_15px_-5px_#0df259] transition-all duration-300 group"
          >
            <FaGithub className="text-xl group-hover:scale-110 transition-transform" />
            <span>GitHub</span>
          </a>
        </div>
      </div>
    </nav>
  );
};

export default Navbar;