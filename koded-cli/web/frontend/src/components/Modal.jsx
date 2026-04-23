import React from 'react';
import { FaTerminal, FaCheckCircle, FaTimes, FaArrowRight } from 'react-icons/fa';

const Modal = ({ isOpen, onClose, position = 1337 }) => {
  if (!isOpen) return null;

  return (
    <>
      {/* Modal Overlay Backdrop */}
      <div className="fixed inset-0 z-50 bg-[#050b07]/80 backdrop-blur-sm transition-all duration-300" />
      
      {/* Modal Container */}
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
        {/* Card Component reused/modified as Modal */}
        <div className="relative w-full max-w-[480px] bg-background-dark border border-primary/20 rounded-xl shadow-neon flex flex-col overflow-hidden animate-in fade-in zoom-in-95 duration-300">
          {/* Top Bar decoration (Terminal style) */}
          <div className="h-1 w-full bg-gradient-to-r from-primary/40 via-primary to-primary/40" />
          
          <div className="p-8 md:p-10 flex flex-col items-center text-center">
            {/* Icon/Indicator Area */}
            <div className="relative mb-6 group">
              <div className="absolute inset-0 bg-primary/20 blur-xl rounded-full scale-150 opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
              <div className="w-16 h-16 rounded-full bg-primary/10 border border-primary/30 flex items-center justify-center relative z-10">
                <FaTerminal className="text-4xl text-primary" />
              </div>
              {/* Small status badge */}
              <div className="absolute -bottom-1 -right-1 bg-background-dark rounded-full p-1 border border-primary/20 z-20">
                <FaCheckCircle className="text-sm text-primary" />
              </div>
            </div>
            
            {/* Text Content */}
            <div className="space-y-4 mb-8">
              <h2 className="text-white tracking-tight text-3xl font-bold leading-tight">
                You're on the waitlist!
              </h2>
              <p className="text-gray-400 text-base font-normal leading-relaxed max-w-xs mx-auto">
                Koded is coming soon. Watch your inbox for your deployment key.
              </p>
              
              {/* MetaText reused */}
              <div className="pt-2">
                <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded bg-primary/5 border border-primary/10">
                  <span className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />
                  <p className="text-primary text-xs font-mono tracking-widest uppercase">
                    System ID: #{position}-KODED-V2
                  </p>
                </div>
              </div>
            </div>
            
            {/* Actions */}
            <div className="w-full space-y-3">
              {/* Primary Button reused */}
              <button
                onClick={onClose}
                className="w-full relative group overflow-hidden rounded-lg h-12 bg-primary text-[#102316] text-sm font-bold tracking-[0.015em] transition-all hover:shadow-[0_0_20px_rgba(13,242,89,0.4)] active:scale-[0.98]"
              >
                <div className="absolute inset-0 bg-white/20 translate-y-full group-hover:translate-y-0 transition-transform duration-300 ease-out" />
                <span className="relative flex items-center justify-center gap-2">
                  Return to Terminal
                  <FaArrowRight className="text-lg" />
                </span>
              </button>
              
              {/* Secondary Ghost Button */}
              <button
                onClick={onClose}
                className="w-full h-10 text-gray-500 hover:text-white text-sm font-medium transition-colors"
              >
                Close this window
              </button>
            </div>
          </div>
          
          {/* Bottom decorative grid fade */}
          <div 
            className="absolute bottom-0 left-0 right-0 h-32 bg-gradient-to-t from-primary/5 to-transparent pointer-events-none opacity-20"
            style={{
              backgroundImage: `
                linear-gradient(0deg, transparent 24%, rgba(13, 242, 89, .3) 25%, rgba(13, 242, 89, .3) 26%, transparent 27%, transparent 74%, rgba(13, 242, 89, .3) 75%, rgba(13, 242, 89, .3) 76%, transparent 77%, transparent),
                linear-gradient(90deg, transparent 24%, rgba(13, 242, 89, .3) 25%, rgba(13, 242, 89, .3) 26%, transparent 27%, transparent 74%, rgba(13, 242, 89, .3) 75%, rgba(13, 242, 89, .3) 76%, transparent 77%, transparent)
              `,
              backgroundSize: '30px 30px'
            }}
          />
        </div>
      </div>
    </>
  );
};

export default Modal;