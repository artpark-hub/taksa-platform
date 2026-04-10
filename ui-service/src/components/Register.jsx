'use client';

import React, { useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Eye, EyeOff } from 'lucide-react'; 

const Register = () => {
    const router = useRouter();
    const [formData, setFormData] = useState({
        firstName: '', lastName: '', email: '', orgName: '', password: '', confirmPassword: ''
    });

    const [showPassword, setShowPassword] = useState(false);
    const [showConfirmPassword, setShowConfirmPassword] = useState(false);

    const [errors, setErrors] = useState({});
    const [isLoading, setIsLoading] = useState(false);
    const [formError, setFormError] = useState('');

    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

    const handleChange = (e) => {
        const { name, value } = e.target;
        setFormData({ ...formData, [name]: value });
        if (errors[name]) setErrors({ ...errors, [name]: '' });

        if (name === 'password') {
            if (value && value.length < 8) {
                setErrors(prev => ({ ...prev, password: "Password must be at least 8 characters" }));
            } else {
                setErrors(prev => ({ ...prev, password: "" }));
            }
        }

        if (name === 'confirmPassword' && formData.password && value !== formData.password) {
            setErrors(prev => ({ ...prev, confirmPassword: "Passwords do not match" }));
        } else if (name === 'confirmPassword') {
            setErrors(prev => ({ ...prev, confirmPassword: "" }));
        }

        if (name === 'password' && formData.confirmPassword && value !== formData.confirmPassword) {
            setErrors(prev => ({ ...prev, confirmPassword: "Passwords do not match" }));
        } else if (name === 'password' && formData.confirmPassword && value === formData.confirmPassword) {
            setErrors(prev => ({ ...prev, confirmPassword: "" }));
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

    const registerWithKratos = async () => {
        const initRes = await fetch('/self-service/registration/browser', {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include',
            redirect: 'follow'
        });

        const initData = await initRes.json().catch(() => ({}));

        if (!initRes.ok) {
            throw new Error(initData?.error?.message || initData?.message || 'Failed to initialize registration flow');
        }

        const flowId = initData?.id;
        if (!flowId) throw new Error('Registration flow id not found');

        const flowRes = await fetch(`/self-service/registration/flows?id=${flowId}`, {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include'
        });

        const flowData = await flowRes.json().catch(() => ({}));

        if (!flowRes.ok) {
            throw new Error(flowData?.error?.message || flowData?.message || 'Failed to fetch registration flow');
        }

        const csrfNode = flowData?.ui?.nodes?.find(n => n?.attributes?.name === 'csrf_token');
        const csrfToken = csrfNode?.attributes?.value || '';

        const registerRes = await fetch(`/self-service/registration?flow=${flowId}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
            credentials: 'include',
            body: JSON.stringify({
                method: 'password',
                password: formData.password,
                traits: {
                    email: formData.email,
                    name: {
                        first: formData.firstName,
                        last: formData.lastName
                    },
                    role: 'master',
                    organization_name: formData.orgName
                },
                csrf_token: csrfToken
            })
        });

        const registerData = await registerRes.json().catch(() => ({}));

        if (!registerRes.ok) {
            throw new Error(
                registerData?.ui?.messages?.[0]?.text ||
                registerData?.error?.message ||
                registerData?.message ||
                'Registration failed'
            );
        }

        return registerData;
    };

    const fetchJwt = async () => {
        const jwtResponse = await fetch('/api/v1/um/token', {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include'
        });

        const jwtData = await jwtResponse.json().catch(() => ({}));

        if (!jwtResponse.ok) {
            throw new Error(jwtData?.message || jwtData?.error?.message || 'Failed to retrieve JWT token');
        }

        const finalJwt =
            jwtData?.data?.jwt_token ||
            jwtData?.jwt_token ||
            jwtData?.data?.jwtToken;

        if (!finalJwt) throw new Error('No JWT token in response');

        localStorage.setItem('taksa_jwt', finalJwt);
    };

    const isFormComplete =
        formData.firstName.trim() &&
        formData.lastName.trim() &&
        formData.email.trim() &&
        formData.orgName.trim() &&
        formData.password.trim() &&
        formData.confirmPassword.trim();

    const isFormValid =
        isFormComplete &&
        formData.password.length >= 8 &&
        emailPattern.test(formData.email) &&
        formData.password === formData.confirmPassword &&
        !errors.email &&
        !errors.password &&
        !errors.confirmPassword;

    const handleSubmit = async (e) => {
        e.preventDefault();
        setFormError('');

        if (!formData.firstName || !formData.lastName || !formData.email || !formData.orgName || !formData.password) {
            setFormError("All fields are required.");
            return;
        }

        if (formData.password.length < 8) {
            setErrors(prev => ({ ...prev, password: "Password must be at least 8 characters" }));
            return;
        }

        if (formData.password !== formData.confirmPassword) {
            setErrors(prev => ({ ...prev, confirmPassword: "Passwords do not match" }));
            return;
        }

        if (errors.email || errors.password || errors.confirmPassword) return;

        setIsLoading(true);

        try {
            const registerData = await registerWithKratos();

            const identity = registerData?.session?.identity;
            const traits = identity?.traits || {};

            if (identity) {
                localStorage.setItem('taksa_user', JSON.stringify({
                    identityId: identity.id,
                    email: traits.email || '',
                    firstName: traits.name?.first || '',
                    lastName: traits.name?.last || '',
                    role: traits.role || '',
                    organizationName: traits.organization_name || ''
                }));
            }

            await fetchJwt();

            router.push('/dashboard');

        } catch (err) {
            console.error("Registration Error:", err);
            setFormError(err.message || "Registration failed");
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="register-main-container">
            <div className="register-left-panel">
                <div className="register-brand-container">
                    <img src="/images/taksa_black.png" alt="Taksa Logo" className="register-brand-logo" />
                    <h3 className="register-brand-title">Factory Operating System</h3>
                </div>
            </div>

            <div className="register-right-panel">
                <div className="register-form-wrapper">
                    <h2 className="register-header-title">Sign Up</h2>

                    {formError && (
                        <div style={{ color: 'red', marginBottom: '15px', padding: '10px', background: '#ffeeee', borderRadius: '4px' }}>
                            {formError}
                        </div>
                    )}

                    <form className="register-auth-form" onSubmit={handleSubmit}>
                        <div className="register-name-row" style={{ display: 'flex', gap: '15px' }}>
                            <div className="register-input-group" style={{ flex: 1 }}>
                                <label>First name *</label>
                                <input type="text" name="firstName" placeholder="John" value={formData.firstName} onChange={handleChange} disabled={isLoading} />
                            </div>
                            <div className="register-input-group" style={{ flex: 1 }}>
                                <label>Last name *</label>
                                <input type="text" name="lastName" placeholder="Doe" value={formData.lastName} onChange={handleChange} disabled={isLoading} />
                            </div>
                        </div>

                        <div className="register-input-group">
                            <label>Organization *</label>
                            <input type="text" name="orgName" placeholder="Your organization" value={formData.orgName} onChange={handleChange} disabled={isLoading} />
                        </div>

                        <div className="register-input-group">
                            <label>Email *</label>
                            <input
                                type="email" name="email" placeholder="user@yourcompany.com"
                                value={formData.email} onChange={handleChange} onBlur={handleBlur}
                                className={errors.email ? 'register-input-error-border' : ''}
                                disabled={isLoading}
                            />
                            {errors.email && <span className="register-error-message">{errors.email}</span>}
                        </div>

                        <div className="register-input-group">
                            <label>Password *</label>
                            {errors.password && <span className="register-error-message" style={{ color: 'red', fontSize: '0.8rem', marginBottom: '4px', display: 'block' }}>{errors.password}</span>}
                            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
                                <input
                                    type={showPassword ? "text" : "password"}
                                    name="password"
                                    value={formData.password}
                                    onChange={handleChange}
                                    disabled={isLoading}
                                    style={{ width: '100%', paddingRight: '45px' }}
                                />
                                <button type="button" onClick={() => setShowPassword(!showPassword)}
                                    style={{ position: 'absolute', right: '10px', background: 'transparent', border: 'none', cursor: 'pointer', color: '#6b7280', display: 'flex', alignItems: 'center' }}>
                                    {showPassword ? <Eye size={20} /> : <EyeOff size={20} />}
                                </button>
                            </div>
                        </div>

                        <div className="register-input-group">
                            <label>Confirm Password *</label>
                            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
                                <input
                                    type={showConfirmPassword ? "text" : "password"}
                                    name="confirmPassword"
                                    value={formData.confirmPassword}
                                    onChange={handleChange}
                                    className={errors.confirmPassword ? 'register-input-error-border' : ''}
                                    disabled={isLoading}
                                    style={{ width: '100%', paddingRight: '45px' }}
                                />
                                <button type="button" onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                                    style={{ position: 'absolute', right: '10px', background: 'transparent', border: 'none', cursor: 'pointer', color: '#6b7280', display: 'flex', alignItems: 'center' }}>
                                    {showConfirmPassword ? <Eye size={20} /> : <EyeOff size={20} />}
                                </button>
                            </div>
                            {errors.confirmPassword && <span className="register-error-message" style={{ color: 'red', fontSize: '0.8rem', marginTop: '4px', display: 'block' }}>{errors.confirmPassword}</span>}
                        </div>

                        <button type="submit" className="register-submit-btn" disabled={isLoading || !isFormValid} style={{ marginTop: '15px' }}>
                            {isLoading ? "Creating Account..." : "Sign Up"}
                        </button>
                    </form>

                    <div style={{ textAlign: 'center', marginTop: '20px', fontSize: '0.9rem', color: '#666' }}>
                        Already have an account? <Link href="/" style={{ color: '#0056b3', fontWeight: '600', textDecoration: 'none' }}>Log in</Link>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Register;