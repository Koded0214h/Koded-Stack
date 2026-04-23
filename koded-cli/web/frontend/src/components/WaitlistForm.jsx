import React, { useState } from 'react';
import { FaDiscord, FaChevronDown } from 'react-icons/fa';

const WaitlistForm = ({ onSubmit, totalUsers = 0, loading = false }) => {
  const [formData, setFormData] = useState({
    name: '',
    email: '',
    source: '',
    newsletter: true
  });

  const handleChange = (e) => {
    const { name, value, type, checked } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value
    }));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    await onSubmit(formData);
    // Reset form on success (form will be reset by parent component after API call)
    if (!loading) {
      setFormData({
        name: '',
        email: '',
        source: '',
        newsletter: true
      });
    }
  };

  // Sample user avatars
  const userAvatars = [
    'https://api.dicebear.com/7.x/avataaars/svg?seed=Taylor&backgroundColor=0a140d',
    'https://api.dicebear.com/7.x/avataaars/svg?seed=Alex&backgroundColor=0a140d',
    'https://api.dicebear.com/7.x/avataaars/svg?seed=Jordan&backgroundColor=0a140d',
    'https://api.dicebear.com/7.x/avataaars/svg?seed=Casey&backgroundColor=0a140d',
    'https://api.dicebear.com/7.x/avataaars/svg?seed=Riley&backgroundColor=0a140d',
  ];

  return (
    <section id="waitlist" className="w-full bg-[#0a140d] border-y border-border-dark py-24 relative overflow-hidden">
      {/* Background Decoration */}
      <div className="absolute right-0 top-0 h-full w-1/3 bg-gradient-to-l from-primary/5 to-transparent pointer-events-none" />
      
      <div className="layout-container max-w-7xl mx-auto px-6 grid lg:grid-cols-2 gap-16 items-center relative z-10">
        {/* Text Content */}
        <div className="flex flex-col gap-6">
          <h2 className="text-4xl font-black text-white tracking-tight">Join the Waitlist</h2>
          <p className="text-lg text-text-muted leading-relaxed">
            We are currently onboarding developers in batches to ensure stability. 
            Sign up now to get early access to the binary and our private Discord community.
          </p>
          
          {/* User Avatars */}
          <div className="flex gap-4 mt-2">
            <div className="flex -space-x-3">
              {userAvatars.map((avatar, index) => (
                <img 
                  key={index}
                  src={avatar}
                  alt={`Developer avatar ${index + 1}`}
                  className="w-10 h-10 rounded-full border-2 border-[#0a140d] bg-gray-600"
                />
              ))}
              <div className="w-10 h-10 rounded-full border-2 border-[#0a140d] bg-surface-dark flex items-center justify-center text-xs font-bold text-white">
                {totalUsers > 2000 ? `+${Math.floor(totalUsers/1000)}k` : `+${totalUsers}`}
              </div>
            </div>
            <div className="flex flex-col justify-center">
              <span className="text-white font-bold text-sm">Developers waiting</span>
              <span className="text-text-muted text-xs">Join the revolution</span>
            </div>
          </div>
        </div>
        
        {/* Form Card */}
        <div className="bg-surface-dark border border-border-dark p-8 rounded-2xl shadow-xl glow-effect">
          <form onSubmit={handleSubmit} className="flex flex-col gap-5">
            <label className="flex flex-col gap-2">
              <span className="text-sm font-bold text-white">Full Name</span>
              <input
                name="name"
                value={formData.name}
                onChange={handleChange}
                required
                disabled={loading}
                className="w-full bg-background-dark border border-border-dark rounded-lg px-4 py-3 text-white placeholder:text-gray-600 focus:border-primary focus:ring-1 focus:ring-primary outline-none transition-all disabled:opacity-50"
                placeholder="Linus Torvalds"
                type="text"
              />
            </label>
            
            <label className="flex flex-col gap-2">
              <span className="text-sm font-bold text-white">Email Address</span>
              <input
                name="email"
                value={formData.email}
                onChange={handleChange}
                required
                disabled={loading}
                className="w-full bg-background-dark border border-border-dark rounded-lg px-4 py-3 text-white placeholder:text-gray-600 focus:border-primary focus:ring-1 focus:ring-primary outline-none transition-all disabled:opacity-50"
                placeholder="dev@example.com"
                type="email"
              />
            </label>
            
            <label className="flex flex-col gap-2">
              <span className="text-sm font-bold text-white">Where did you hear about Koded?</span>
              <div className="relative">
                <select
                  name="source"
                  value={formData.source}
                  onChange={handleChange}
                  required
                  disabled={loading}
                  className="w-full bg-background-dark border border-border-dark rounded-lg px-4 py-3 text-white appearance-none focus:border-primary focus:ring-1 focus:ring-primary outline-none transition-all cursor-pointer disabled:opacity-50"
                >
                  <option value="" disabled>Select an option</option>
                  <option value="twitter">Twitter / X</option>
                  <option value="github">GitHub</option>
                  <option value="friend">Friend / Colleague</option>
                  <option value="blog">Tech Blog</option>
                  <option value="other">Other</option>
                </select>
                <FaChevronDown className="absolute right-4 top-1/2 -translate-y-1/2 text-text-muted pointer-events-none" />
              </div>
            </label>
            
            <label className="flex items-center gap-3 mt-2 cursor-pointer group">
              <input
                name="newsletter"
                checked={formData.newsletter}
                onChange={handleChange}
                disabled={loading}
                className="w-5 h-5 rounded border-border-dark bg-background-dark text-primary focus:ring-primary focus:ring-offset-background-dark disabled:opacity-50"
                type="checkbox"
              />
              <span className="text-sm text-text-muted group-hover:text-white transition-colors">
                I want to receive news and updates
              </span>
            </label>
            
            <button
              type="submit"
              disabled={loading}
              className="mt-4 w-full bg-primary hover:bg-white text-background-dark font-bold text-center py-4 rounded-lg cursor-pointer transition-all transform active:scale-[0.98] select-none shadow-[0_0_20px_-5px_rgba(13,242,89,0.5)] disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? (
                <span className="flex items-center justify-center gap-2">
                  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                  </svg>
                  Processing...
                </span>
              ) : (
                'Join Waitlist'
              )}
            </button>
          </form>
        </div>
      </div>
    </section>
  );
};

export default WaitlistForm;