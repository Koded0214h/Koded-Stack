import React, { useState, useEffect } from 'react';
import { waitlistAPI } from './services/api';
import Navbar from './components/Navbar';
import Hero from './components/Hero';
import Features from './components/Features';
import WaitlistForm from './components/WaitlistForm';
import Footer from './components/Footer';
import Modal from './components/Modal';

function App() {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [waitlistPosition, setWaitlistPosition] = useState(null);
  const [totalUsers, setTotalUsers] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  // Fetch total users on mount
  useEffect(() => {
    fetchTotalUsers();
  }, []);

  const fetchTotalUsers = async () => {
    try {
      const data = await waitlistAPI.getTotalUsers();
      setTotalUsers(data.total_users || 0);
    } catch (err) {
      console.error('Failed to fetch total users:', err);
      // Fallback to static number
      setTotalUsers(2100);
    }
  };

  const handleFormSubmit = async (formData) => {
    setLoading(true);
    setError(null);

    try {
      // Map form data to API format
      const apiData = {
        email: formData.email,
        name: formData.name,
        referral_source: formData.source || 'other',
        wants_updates: formData.newsletter,
      };

      // Submit to backend
      const response = await waitlistAPI.joinWaitlist(apiData);
      
      // Update position and show modal
      setWaitlistPosition(response.position);
      setIsModalOpen(true);
      
      // Refresh total users count
      fetchTotalUsers();
      
    } catch (err) {
      setError(err.message || 'Failed to join waitlist. Please try again.');
      console.error('Waitlist submission error:', err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="dark bg-background-dark font-display text-white antialiased overflow-x-hidden relative flex flex-col min-h-screen">
      <Navbar />
      <main className="flex-grow flex flex-col items-center w-full">
        <Hero />
        <Features />
        
        {/* Error Display */}
        {error && (
          <div className="w-full max-w-7xl px-6 mb-4">
            <div className="bg-red-900/30 border border-red-500/50 rounded-lg p-4 text-red-200">
              <p className="font-medium">⚠️ {error}</p>
              <p className="text-sm opacity-80 mt-1">
                Please check your information and try again.
              </p>
            </div>
          </div>
        )}
        
        <WaitlistForm 
          onSubmit={handleFormSubmit} 
          totalUsers={totalUsers}
          loading={loading}
        />
      </main>
      <Footer />
      <Modal 
        isOpen={isModalOpen} 
        onClose={() => setIsModalOpen(false)}
        position={waitlistPosition}
      />
    </div>
  );
}

export default App;