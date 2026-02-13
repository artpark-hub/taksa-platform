import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import './Register.css';
import taksaBlackLogo from '../assets/images/taksa_black.png';

const Register = () => {
    const navigate = useNavigate();

    const [formData, setFormData] = useState({
        firstName: '', lastName: '', email: '', orgName: '', password: '', confirmPassword: ''
    });
    const [errors, setErrors] = useState({});

    // New state for API handling
    const [isLoading, setIsLoading] = useState(false);
    const [formError, setFormError] = useState('');

    // --- Configuration ---
    const KRATOS_PUBLIC_URL = "http://172.25.60.23:4433";
    const OATHKEEPER_PROXY_URL = "http://172.25.60.23:4457";

    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

    const handleChange = (e) => {
        const { name, value } = e.target;
        setFormData({ ...formData, [name]: value });
        if (errors[name]) setErrors({ ...errors, [name]: '' });

        if (name === 'confirmPassword' && formData.password && value !== formData.password) {
            setErrors(prev => ({ ...prev, confirmPassword: "Passwords do not match" }));
        } else if (name === 'confirmPassword') {
            setErrors(prev => ({ ...prev, confirmPassword: "" }));
        }

        if (name === 'password' && formData.confirmPassword && value !== formData.confirmPassword) {
            setErrors(prev => ({ ...prev, confirmPassword: "Passwords do not match" }));
        }
    };

    const handleBlur = (e) => {
        const { name, value } = e.target;
        if (name === 'email') {
            if (value && !emailPattern.test(value)) {
                setErrors(prev => ({ ...prev, email: "Please enter a valid email address" }));
            }
        }
    };

    const handleSubmit = async (e) => {
        e.preventDefault();
        setFormError('');

        // 1. Validation
        if (!formData.firstName || !formData.lastName || !formData.email || !formData.orgName || !formData.password) {
            setFormError("All fields are required.");
            return;
        }
        if (formData.password !== formData.confirmPassword) {
            setErrors(prev => ({ ...prev, confirmPassword: "Passwords do not match" }));
            return;
        }
        if (errors.email || errors.confirmPassword) {
            return;
        }

        setIsLoading(true);

        try {
            // -----------------------------------------------------------
            // Step 1: Register User (Master User Registration)
            // -----------------------------------------------------------
            console.log("Step 1: Registering User...", formData);

            const registerResponse = await fetch(`${OATHKEEPER_PROXY_URL}/api/v1/um/register_master_user`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    email: formData.email,
                    password: formData.password,
                    first_name: formData.firstName,
                    last_name: formData.lastName,
                    organization_name: formData.orgName
                })
            });

            if (!registerResponse.ok) {
                const errorData = await registerResponse.json();
                throw new Error(errorData.message || "Registration failed");
            }

            const registerData = await registerResponse.json();
            console.log("Registration Successful:", registerData);

            // -----------------------------------------------------------
            // Step 2: Auto-Login via Direct API
            // -----------------------------------------------------------
            console.log("Step 2: Auto-Login via Direct API...");

            const loginResponse = await fetch(`${OATHKEEPER_PROXY_URL}/api/v1/um/login`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    email: formData.email,
                    password: formData.password
                })
            });

            const loginData = await loginResponse.json();

            if (!loginResponse.ok) {
                console.error("Auto-login failed:", loginData);
                throw new Error(loginData.message || "Auto-login failed after registration.");
            }

            console.log("Auto-Login Successful.", loginData);

            // =========================================================
            // 1. STORE USER DETAILS (Name, Email, Role)
            // =========================================================
            if (loginData.user) {
                localStorage.setItem('taksa_user', JSON.stringify(loginData.user));
                console.log("✅ User details saved to Local Storage.");
            }

            // Get session token for next step (don't save to local storage, just use it)
            const sessionToken = loginData.sessionToken;

            // -----------------------------------------------------------
            // Step 3: Exchange Session Token for JWT
            // -----------------------------------------------------------
            console.log("Step 3: Getting JWT...");

            const jwtResponse = await fetch(`${OATHKEEPER_PROXY_URL}/api/v1/um/token`, {
                method: 'GET',
                headers: {
                    'Accept': 'application/json',
                    // Use the session token if available
                    ...(sessionToken ? { 'Authorization': `Bearer ${sessionToken}` } : {})
                },
                credentials: 'include'
            });

            if (!jwtResponse.ok) {
                const jwtError = await jwtResponse.json().catch(() => ({}));
                console.error("JWT exchange failed:", jwtError);
                throw new Error('Failed to retrieve JWT token.');
            }

            const jwtData = await jwtResponse.json();
            const finalJwt = jwtData.data?.jwtToken || jwtData.data?.jwt_token || jwtData.jwt_token;

            if (!finalJwt) {
                console.error("JWT not found in response. Keys:", Object.keys(jwtData));
                throw new Error('No JWT token in response');
            }

            // =========================================================
            // 2. STORE JWT TOKEN (Crucial for API Access)
            // =========================================================
            localStorage.setItem('taksa_jwt', finalJwt);
            console.log("✅ Registration & Auto-Login Complete! JWT Saved.");

            // Redirect
            navigate('/Dashboard');

        } catch (err) {
            console.error("Registration/Login Error:", err);
            setFormError(err.message || "An unexpected error occurred.");
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="register-page">
            {/* 1. LEFT PANEL: Branding & Background */}
            <div className="register-left-panel">
                <div className="brand-container">
                    <img src={taksaBlackLogo} alt="Taksa Logo" className="brand-logo" />
                    <h3 className="brand-title">Factory Operating System</h3>
                </div>
            </div>

            {/* 2. RIGHT PANEL: Transparent wrapper */}
            <div className="register-right-panel">
                {/* 3. THE CARD: White background with shadow */}
                <div className="register-form-container">
                    <h2 className="form-header">Sign Up</h2>
                    <p className="form-subtext">Start creating your Unified Namespace — create your account.</p>

                    {formError && (
                        <div style={{ color: 'red', marginBottom: '15px', padding: '10px', background: '#ffeeee', borderRadius: '4px' }}>
                            {formError}
                        </div>
                    )}

                    <form className="register-form" onSubmit={handleSubmit}>
                        <div className="name-row">
                            <div className="form-group">
                                <label>First name *</label>
                                <input type="text" name="firstName" placeholder="John" value={formData.firstName} onChange={handleChange} disabled={isLoading} />
                            </div>
                            <div className="form-group">
                                <label>Last name *</label>
                                <input type="text" name="lastName" placeholder="Doe" value={formData.lastName} onChange={handleChange} disabled={isLoading} />
                            </div>
                        </div>

                        <div className="form-group">
                            <label>Company *</label>
                            <input type="text" name="orgName" placeholder="Your company" value={formData.orgName} onChange={handleChange} disabled={isLoading} />
                        </div>

                        <div className="form-group">
                            <label>Email *</label>
                            <input
                                type="email" name="email" placeholder="user@yourcompany.com"
                                value={formData.email} onChange={handleChange} onBlur={handleBlur}
                                className={errors.email ? 'input-error' : ''}
                                disabled={isLoading}
                            />
                            {errors.email && <span className="error-text">{errors.email}</span>}
                        </div>

                        <div className="form-group">
                            <label>Password *</label>
                            <input type="password" name="password" value={formData.password} onChange={handleChange} disabled={isLoading} />
                        </div>

                        <div className="form-group">
                            <label>Confirm Password *</label>
                            <input
                                type="password" name="confirmPassword" value={formData.confirmPassword} onChange={handleChange}
                                className={errors.confirmPassword ? 'input-error' : ''}
                                disabled={isLoading}
                            />
                            {errors.confirmPassword && <span className="error-text">{errors.confirmPassword}</span>}
                        </div>

                        <button type="submit" className="register-btn" disabled={isLoading}>
                            {isLoading ? "Creating Account..." : "Sign Up"}
                        </button>
                    </form>
                </div>
            </div>
        </div>
    );
};

export default Register;