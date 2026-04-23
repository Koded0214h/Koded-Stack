import React from 'react';
import { 
  FaPauseCircle, 
  FaBolt, 
  FaShieldAlt, 
  FaDesktop 
} from 'react-icons/fa';

const FeatureCard = ({ icon: Icon, title, description }) => (
  <div className="p-6 rounded-2xl bg-surface-dark border border-white/5 hover:border-primary/50 transition-colors group feature-card">
    <div className="w-12 h-12 bg-primary/10 rounded-lg flex items-center justify-center mb-4 group-hover:bg-primary group-hover:text-background-dark transition-colors text-primary">
      <Icon className="w-6 h-6" />
    </div>
    <h3 className="text-xl font-bold text-white mb-2">{title}</h3>
    <p className="text-text-muted text-sm leading-relaxed">{description}</p>
  </div>
);

const Features = () => {
  const features = [
    {
      icon: FaPauseCircle,
      title: "Pause & Resume",
      description: "Network flakey? Pause your install and resume exactly where you left off, even days later."
    },
    {
      icon: FaBolt,
      title: "Hyper Speed",
      description: "Parallelized downloads and optimized caching make Koded up to 5x faster than npm or yarn."
    },
    {
      icon: FaShieldAlt,
      title: "Secure Core",
      description: "Every package is scanned in real-time. We sandbox execution scripts to keep your machine safe."
    },
    {
      icon: FaDesktop,
      title: "Cross Platform",
      description: "Works identically on macOS, Linux, and Windows. One config file to rule them all."
    }
  ];

  return (
    <section className="w-full max-w-7xl mx-auto px-6 py-20 border-t border-border-dark">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {features.map((feature, index) => (
          <FeatureCard key={index} {...feature} />
        ))}
      </div>
    </section>
  );
};

export default Features;