'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Eye, EyeOff } from 'lucide-react'; 

const Login = () => {
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [showPassword, setShowPassword] = useState(false);
    const [emailError, setEmailError] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [formError, setFormError] = useState('');

    const router = useRouter();

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

    const getErrorMessage = (data, fallback) => {
        return (
            data?.ui?.messages?.[0]?.text ||
            data?.error?.message ||
            data?.message ||
            fallback
        );
    };

    const extractCsrfToken = (flow) => {
        const nodes = flow?.ui?.nodes || [];
        const csrfNode = nodes.find(
            (node) => node?.attributes?.name === 'csrf_token'
        );
        return csrfNode?.attributes?.value || '';
    };

    const isRecoverableFlowError = (response, data) => {
        const msg = String(getErrorMessage(data, '')).toLowerCase();
        const errorId = String(data?.error?.id || '').toLowerCase();

        return (
            response.status === 404 ||
            response.status === 410 ||
            errorId.includes('self_service_flow_expired') ||
            errorId.includes('security_csrf_violation') ||
            msg.includes('expired') ||
            msg.includes('csrf') ||
            msg.includes('unable to locate') ||
            msg.includes('not found')
        );
    };

    const isAlreadyLoggedInError = (data) => {
        const msg = String(getErrorMessage(data, '')).toLowerCase();
        const errorId = String(data?.error?.id || '').toLowerCase();

        return (
            msg.includes('already logged in') ||
            msg.includes('session already available') ||
            errorId.includes('session_already_available')
        );
    };

    const createFreshLoginFlow = async () => {
        const response = await fetch('/self-service/login/browser', {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include',
            redirect: 'follow'
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            if (isAlreadyLoggedInError(data)) {
                router.push('/dashboard');
                return null;
            }

            throw new Error(getErrorMessage(data, 'Failed to initialize login flow'));
        }

        const flowId = data?.id;
        if (!flowId) {
            throw new Error('Login flow id not found');
        }

        return {
            flowId,
            csrfToken: extractCsrfToken(data)
        };
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
            setFormError('Please enter your password');
            return;
        }

        setIsLoading(true);

        try {
            const freshFlow = await createFreshLoginFlow();
            if (!freshFlow) return;

            const response = await fetch(`/self-service/login?flow=${encodeURIComponent(freshFlow.flowId)}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    method: 'password',
                    identifier: email.trim(),
                    password: password,
                    csrf_token: freshFlow.csrfToken
                })
            });

            const data = await response.json().catch(() => ({}));

            if (response.ok) {
                const identity = data?.session?.identity;
                const traits = identity?.traits || {};

                if (identity) {
                    localStorage.setItem(
                        'taksa_user',
                        JSON.stringify({
                            // snake_case keys (backwards compatibility)
                            identity_id: identity.id,
                            email: traits.email || '',
                            first_name: traits.name?.first || '',
                            last_name: traits.name?.last || '',
                            role: traits.role || '',
                            organization_name: traits.organization_name || '',
                            // camelCase keys (used by other parts of the app)
                            identityId: identity.id,
                            firstName: traits.name?.first || '',
                            lastName: traits.name?.last || '',
                            role: traits.role || '',
                            organizationName: traits.organization_name || ''
                        })
                    );
                }

                try {
                    const jwtResponse = await fetch('/api/v1/um/token', {
                        method: 'GET',
                        headers: {
                            Accept: 'application/json'
                        },
                        credentials: 'include'
                    });

                    const jwtData = await jwtResponse.json().catch(() => ({}));

                    if (jwtResponse.ok) {
                        const finalJwt =
                            jwtData?.data?.jwt_token ||
                            jwtData?.jwt_token ||
                            jwtData?.data?.jwtToken;

                        if (finalJwt) {
                            localStorage.setItem('taksa_jwt', finalJwt);
                        }
                    }
                } catch (jwtErr) {
                    console.error("JWT fetch error:", jwtErr);
                }

                router.push('/dashboard');
                return;
            }

            if (isAlreadyLoggedInError(data)) {
                router.push('/dashboard');
                return;
            }

            if (isRecoverableFlowError(response, data)) {
                setFormError('Your login session expired. Please try again.');
                return;
            }

            const msg = getErrorMessage(data, 'Login failed');
            const lowerMsg = msg.toLowerCase();

            if (
                response.status === 400 ||
                response.status === 401 ||
                lowerMsg.includes('invalid credentials') ||
                lowerMsg.includes('invalid email or password') ||
                lowerMsg.includes('password') ||
                lowerMsg.includes('credentials') ||
                lowerMsg.includes('identifier')
            ) {
                setFormError('Invalid email or password.');
                return;
            }

            throw new Error(msg);
        } catch (err) {
            console.error("Login Error:", err);
            setFormError(err.message || "An unexpected error occurred.");
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="login-main-container">
            <div className="login-left-panel">
                <div className="login-brand-container">
                    <img src="/images/taksa_black.png" alt="Taksa Logo" className="login-brand-logo" />
                    <h3 className="login-brand-title">Factory Operating System</h3>
                </div>
            </div>

            <div className="login-right-panel">
                <div className="login-form-wrapper">
                    <h2 className="login-header-title">Login</h2>
                    <p className="login-header-subtitle">Access your account by entering your login details below.</p>

                    {formError && (
                        <div style={{ color: 'red', marginBottom: '15px', padding: '10px', background: '#ffeeee', borderRadius: '4px' }}>
                            {formError}
                        </div>
                    )}

                    <form className="login-auth-form" onSubmit={handleLogin}>
                        <div className="login-input-group">
                            <label htmlFor="email">Email *</label>
                            <input
                                type="email"
                                id="email"
                                placeholder="user@yourcompany.com"
                                value={email}
                                onChange={(e) => {
                                    setEmail(e.target.value);
                                    if (emailError) setEmailError('');
                                    if (formError) setFormError('');
                                }}
                                onBlur={handleEmailBlur}
                                className={emailError ? 'login-input-error-border' : ''}
                                disabled={isLoading}
                            />
                            {emailError && <span className="login-error-message">{emailError}</span>}
                        </div>

                        <div className="login-input-group">
                            <label htmlFor="password">Password *</label>
                            <div className="login-password-wrapper" style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
                                <input
                                    type={showPassword ? "text" : "password"}
                                    id="password"
                                    placeholder="Enter password"
                                    value={password}
                                    onChange={(e) => {
                                        setPassword(e.target.value);
                                        if (formError) setFormError('');
                                    }}
                                    disabled={isLoading}
                                    style={{ paddingRight: '45px', width: '100%' }}
                                />
                                <button
                                    type="button"
                                    className="login-password-toggle"
                                    onClick={() => setShowPassword(!showPassword)}
                                    title={showPassword ? "Hide password" : "Show password"}
                                    style={{ position: 'absolute', right: '10px', background: 'transparent', border: 'none', cursor: 'pointer', color: '#6b7280', display: 'flex', alignItems: 'center' }}
                                >
                                    {showPassword ? <Eye size={20} /> : <EyeOff size={20} />}
                                </button>
                            </div>
                        </div>

                        <button type="submit" className="login-submit-btn" disabled={isLoading}>
                            {isLoading ? "Signing In..." : "Sign In"}
                        </button>
                    </form>

                    <div className="login-footer-section">
                        <span>Don't have an account? <Link href="/register">Sign up now</Link></span>
                        <Link href="/recovery" style={{ fontSize: '0.85rem' }}>Forgot Password?</Link>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Login;