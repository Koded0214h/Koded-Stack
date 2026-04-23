const API_BASE_URL = 'https://koded-cli.onrender.com/api';

// Helper function for API requests
async function fetchAPI(endpoint, options = {}) {
  const url = `${API_BASE_URL}${endpoint}`;
  
  const defaultOptions = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const config = { ...defaultOptions, ...options };

  try {
    const response = await fetch(url, config);
    
    if (!response.ok) {
      const error = await response.json().catch(() => ({
        detail: `HTTP error! status: ${response.status}`,
      }));
      throw new Error(error.detail || 'Something went wrong');
    }

    return await response.json();
  } catch (error) {
    console.error('API Error:', error);
    throw error;
  }
}

// Waitlist API functions
export const waitlistAPI = {
  // Join waitlist
  joinWaitlist: async (userData) => {
    return fetchAPI('/waitlist/users/', {
      method: 'POST',
      body: JSON.stringify(userData),
    });
  },

  // Check if email is on waitlist
  checkEmail: async (email) => {
    return fetchAPI(`/waitlist/users/check/?email=${encodeURIComponent(email)}`);
  },

  // Get waitlist stats
  getStats: async () => {
    return fetchAPI('/waitlist/users/stats/');
  },

  // Get total users
  getTotalUsers: async () => {
    return fetchAPI('/waitlist/total/');
  },

  // Get all users (for admin)
  getUsers: async (skip = 0, limit = 100) => {
    return fetchAPI(`/waitlist/users/?skip=${skip}&limit=${limit}`);
  },
};