import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import './Login.css';
import taksaLogo from '../assets/images/taksa_black.png';

const Login = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [emailError, setEmailError] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [formError, setFormError] = useState('');

  const navigate = useNavigate();

  // --- Configuration ---
  const KRATOS_PUBLIC_URL = "http://172.25.60.23:4433";
  const OATHKEEPER_PROXY_URL = "http://172.25.60.23:4457";

  const validateEmail = (email) => {
    const pattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return pattern.test(email);
  };

  const handleEmailBlur = () => {
    if (!email) {
      setEmailError('');
      return;
    }
    if (!validateEmail(email)) {
      setEmailError('Please enter a valid email address');
    } else {
      setEmailError('');
    }
  };

  const handleLogin = async (e) => {
    e.preventDefault();
    setFormError('');

    if (!email) {
      setEmailError('Email is required');
      return;
    } else if (!validateEmail(email)) {
      setEmailError('Please enter a valid email address');
      return;
    }
    if (!password) {
      alert("Please enter your password");
      return;
    }

    setIsLoading(true);

    try {
      console.log("Step 1: Attempting login...");

      // --- STEP 1: LOGIN CALL ---
      const response = await fetch(`${OATHKEEPER_PROXY_URL}/api/v1/um/login`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          email: email,
          password: password
        })
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.message || "Login failed");
      }

      console.log("Login Successful!", data);

      // =========================================================
      // 1. STORE USER DETAILS (Name, Email, Role)
      // =========================================================
      if (data.user) {
        localStorage.setItem('taksa_user', JSON.stringify(data.user));
        console.log("✅ User details saved to Local Storage.");
      }

      // We grab the sessionToken for the next step, but DO NOT save it to localStorage
      const sessionToken = data.sessionToken;

      // --- STEP 2: EXCHANGE SESSION TOKEN FOR JWT ---
      try {
        console.log("Step 2: Getting JWT Token...");
        const jwtResponse = await fetch(`${OATHKEEPER_PROXY_URL}/api/v1/um/token`, {
          method: 'GET',
          headers: {
            'Accept': 'application/json',
            ...(sessionToken ? { 'Authorization': `Bearer ${sessionToken}` } : {})
          },
          credentials: 'include'
        });

        if (!jwtResponse.ok) {
          throw new Error(`JWT Exchange failed: ${jwtResponse.status}`);
        }

        const jwtData = await jwtResponse.json();
        const finalJwt = jwtData.data?.jwtToken || jwtData.data?.jwt_token || jwtData.jwt_token;

        if (!finalJwt) {
          throw new Error("JWT token missing in response");
        }

        // =========================================================
        // 2. STORE JWT TOKEN (Crucial for API Access)
        // =========================================================
        localStorage.setItem('taksa_jwt', finalJwt);
        console.log("✅ JWT Token saved to Local Storage.");

        // Redirect to Dashboard
        navigate('/data-flow');

      } catch (jwtErr) {
        console.error("Step 2 Failed:", jwtErr);
        alert(`Login successful, but Token Exchange failed: ${jwtErr.message}`);
      }

    } catch (err) {
      console.error("Login Error:", err);
      setFormError(err.message || "An unexpected error occurred.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-left-panel">
        <div className="brand-container">
          <img src={taksaLogo} alt="Taksa Logo" className="brand-logo" />
          <h3 className="brand-title">Factory Operating System</h3>
        </div>
      </div>

      <div className="login-right-panel">
        <div className="login-form-container">
          <h2 className="form-header">Login</h2>
          <p className="form-subtext">Access your account by entering your login details below.</p>

          {formError && (
            <div style={{ color: 'red', marginBottom: '15px', padding: '10px', background: '#ffeeee', borderRadius: '4px' }}>
              {formError}
            </div>
          )}

          <form className="login-form" onSubmit={handleLogin}>
            <div className="form-group">
              <label htmlFor="email">Email *</label>
              <input
                type="email"
                id="email"
                placeholder="user@yourcompany.com"
                value={email}
                onChange={(e) => {
                  setEmail(e.target.value);
                  if (emailError) setEmailError('');
                }}
                onBlur={handleEmailBlur}
                className={emailError ? 'input-error' : ''}
                disabled={isLoading}
              />
              {emailError && <span className="error-text">{emailError}</span>}
            </div>

            <div className="form-group">
              <label htmlFor="password">Password *</label>
              <input
                type="password"
                id="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={isLoading}
              />
            </div>

            <button type="submit" className="signin-btn" disabled={isLoading}>
              {isLoading ? "Signing In..." : "Sign In"}
            </button>
          </form>

          <div className="login-footer">
            <span>Don't have an account? <Link to="/Register">Sign up now</Link></span>
            <Link to="/forgot-password" style={{ fontSize: '0.85rem' }}>Forgot Password?</Link>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Login;