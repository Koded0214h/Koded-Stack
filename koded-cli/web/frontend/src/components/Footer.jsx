import React from 'react';
import { FaTwitter, FaGithub, FaTerminal } from 'react-icons/fa';

const Footer = () => {
  const currentYear = new Date().getFullYear();
  
  return (
    <footer className="w-full border-t border-border-dark bg-background-dark pt-5 pb-5 px-6">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row justify-between items-center gap-6">
        <div className="flex items-center gap-2">
          <FaTerminal className="text-primary" />
          <span className="text-lg font-bold text-white tracking-tight">Koded_</span>
          <span className="text-text-muted text-sm ml-4">© {currentYear} Koded CLI Inc.</span>
        </div>
        
        <div className="flex items-center gap-8">
          <a className="text-text-muted hover:text-primary transition-colors text-sm font-medium" href="#">
            Privacy
          </a>
          <a className="text-text-muted hover:text-primary transition-colors text-sm font-medium" href="#">
            Terms
          </a>
          <a className="text-text-muted hover:text-primary transition-colors text-sm font-medium" href="#">
            <FaTwitter className="inline w-4 h-4 mr-1" />
            Twitter
          </a>
          <a 
            href="https://github.com/Koded0214h/koded-cli"
            target="_blank"
            rel="noopener noreferrer"
            className="text-text-muted hover:text-primary transition-colors text-sm font-medium"
          >
            <FaGithub className="inline w-4 h-4 mr-1" />
            GitHub
          </a>
        </div>
      </div>
    </footer>
  );
};

export default Footer;