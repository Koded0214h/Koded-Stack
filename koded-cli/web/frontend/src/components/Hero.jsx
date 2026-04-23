import React from 'react';
import { FaRocket, FaBook, FaArrowRight } from 'react-icons/fa';

const Hero = () => {
  const scrollToWaitlist = () => {
    document.getElementById('waitlist').scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <section className="relative w-full pt-20 pb-32 px-6 flex flex-col items-center overflow-hidden">
      {/* Decorative Background Grid */}
      <div className="absolute inset-0 bg-[size:40px_40px] bg-grid-pattern opacity-10 pointer-events-none" />
      
      {/* Glow Effects */}
      <div className="absolute top-20 left-1/4 w-96 h-96 bg-primary/10 rounded-full blur-[100px] pointer-events-none" />
      <div className="absolute bottom-20 right-1/4 w-64 h-64 bg-primary/10 rounded-full blur-[80px] pointer-events-none" />
      
      <div className="relative max-w-5xl w-full flex flex-col items-center text-center z-10">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-primary text-xs font-bold tracking-wide uppercase mb-8">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary opacity-75" />
            <span className="relative inline-flex rounded-full h-2 w-2 bg-primary" />
          </span>
          v1.0 Public Beta Soon
        </div>
        
        <h1 className="text-5xl md:text-7xl font-black tracking-tighter leading-[1.1] mb-6 max-w-4xl text-white">
          The CLI that puts you in <br className="hidden md:block" />
          <span className="text-transparent bg-clip-text bg-gradient-to-r from-primary via-white to-primary bg-[length:200%_auto] animate-gradient">
            control
          </span> of installations.
        </h1>
        
        <p className="text-lg md:text-xl text-text-muted max-w-2xl mb-12 leading-relaxed">
          Pause downloads. Resume anywhere. Cross-platform speed. <br className="hidden sm:block" />
          Say goodbye to broken dependencies and half-finished builds.
        </p>

        {/* CTA Buttons */}
        <div className="flex flex-col sm:flex-row gap-4 mb-12">
          <button
            onClick={scrollToWaitlist}
            className="inline-flex items-center justify-center gap-3 px-8 py-4 bg-primary hover:bg-white text-background-dark font-bold rounded-lg transition-all transform hover:scale-[1.02] active:scale-[0.98] shadow-[0_0_25px_-8px_rgba(13,242,89,0.4)]"
          >
            <FaRocket className="w-5 h-5" />
            Get Early Access
          </button>
          <a
            href="#documentation"
            className="inline-flex items-center justify-center gap-3 px-8 py-4 bg-surface-dark border border-border-dark hover:border-primary/50 text-white font-bold rounded-lg transition-all hover:shadow-[0_0_15px_-5px_rgba(13,242,89,0.3)] group"
          >
            <FaBook className="w-5 h-5" />
            Documentation
            <FaArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
          </a>
        </div>
        
        {/* Terminal Demo */}
        <div className="w-full max-w-4xl bg-[#0a0a0a] rounded-xl border border-border-dark shadow-2xl overflow-hidden">
          <div className="flex items-center gap-2 px-4 py-3 bg-surface-dark border-b border-border-dark">
            <div className="flex gap-2">
              <div className="w-3 h-3 rounded-full bg-red-500/50" />
              <div className="w-3 h-3 rounded-full bg-yellow-500/50" />
              <div className="w-3 h-3 rounded-full bg-green-500/50" />
            </div>
            <div className="flex-1 text-center text-xs text-text-muted font-mono opacity-50">
              user@dev-machine:~
            </div>
          </div>
          
          <div className="p-6 text-left font-mono text-sm md:text-base bg-black/50">
            <div className="flex gap-3 mb-2">
              <span className="text-primary font-bold">➜</span>
              <span className="text-white">koded install --optimize react-native</span>
            </div>
            
            <div className="text-text-muted mb-4">
              Analyzing dependencies... <span className="text-primary">Done (0.4s)</span><br />
              Optimization engine active.
            </div>
            
            <div className="flex flex-col gap-2 mb-2">
              <div className="flex items-center gap-4 text-xs text-text-muted">
                <span className="w-24">core-pkg</span>
                <div className="h-1.5 flex-1 bg-surface-dark rounded-full overflow-hidden">
                  <div className="h-full bg-primary w-[100%]" />
                </div>
                <span>100%</span>
              </div>
              
              <div className="flex items-center gap-4 text-xs text-text-muted">
                <span className="w-24">native-bridge</span>
                <div className="h-1.5 flex-1 bg-surface-dark rounded-full overflow-hidden">
                  <div className="h-full bg-primary w-[75%] animate-pulse" />
                </div>
                <span>75%</span>
              </div>
              
              <div className="flex items-center gap-4 text-xs text-text-muted">
                <span className="w-24">assets</span>
                <div className="h-1.5 flex-1 bg-surface-dark rounded-full overflow-hidden">
                  <div className="h-full bg-primary/30 w-[15%]" />
                </div>
                <span className="text-primary cursor-pointer hover:underline">[ PAUSED ]</span>
              </div>
            </div>
            
            {/* Blinking cursor */}
            <div className="flex gap-3 text-text-muted">
              <span className="text-primary font-bold">➜</span>
              <span className="flex items-center">
                <span className="w-2 h-4 bg-text-muted inline-block animate-blink" />
              </span>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Hero;