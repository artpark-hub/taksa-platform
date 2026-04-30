'use client';

import React, { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Eye, EyeOff } from 'lucide-react'; 

const GOOGLE_LOGIN_ATTEMPT_KEY = 'taksa_google_login_attempt';
const GOOGLE_LOGIN_ERROR_KEY = 'taksa_google_login_error';

const Login = () => {
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [showPassword, setShowPassword] = useState(false);
    const [emailError, setEmailError] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [formError, setFormError] = useState('');
    const [agreementChecked, setAgreementChecked] = useState(false);
    const [agreementError, setAgreementError] = useState('');
    const [openSection, setOpenSection] = useState('social');

    const router = useRouter();

    const validateEmail = (value) => {
        const pattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        return pattern.test(value);
    };

    const clearStoredAuth = () => {
        localStorage.removeItem('taksa_user');
        localStorage.removeItem('taksa_jwt');
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

    const appendHiddenInput = (form, name, value) => {
        const input = document.createElement('input');
        input.type = 'hidden';
        input.name = name;
        input.value = value;
        form.appendChild(input);
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

    const hasRequiredOrgTraits = (traits = {}) => {
        return Boolean(
            String(traits?.organization_name || '').trim() &&
            String(traits?.tenant_id || '').trim()
        );
    };

    const storeUserInLocalStorage = (identity) => {
        const traits = identity?.traits || {};

        localStorage.setItem(
            'taksa_user',
            JSON.stringify({
                identity_id: identity.id,
                email: traits.email || '',
                first_name: traits.name?.first || '',
                last_name: traits.name?.last || '',
                role: traits.role || '',
                organization_name: traits.organization_name || '',
                tenant_id: traits.tenant_id || '',
                identityId: identity.id,
                firstName: traits.name?.first || '',
                lastName: traits.name?.last || '',
                organizationName: traits.organization_name || '',
                tenantId: traits.tenant_id || ''
            })
        );
    };

    const fetchJwt = async () => {
        const jwtResponse = await fetch('/api/v1/um/token', {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
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

        if (!finalJwt) {
            throw new Error('No JWT token in response');
        }

        localStorage.setItem('taksa_jwt', finalJwt);
    };

    const handleAuthenticatedIdentity = async (identity) => {
        const traits = identity?.traits || {};

        if (!identity) {
            throw new Error('Logged in user identity not found');
        }

        if (!hasRequiredOrgTraits(traits)) {
            clearStoredAuth();
            setFormError('Login failed. Your account is missing organisation details. Please complete registration first.');
            return false;
        }

        sessionStorage.removeItem(GOOGLE_LOGIN_ATTEMPT_KEY);
        sessionStorage.removeItem(GOOGLE_LOGIN_ERROR_KEY);

        storeUserInLocalStorage(identity);

        try {
            await fetchJwt();
        } catch (jwtErr) {
            console.error('JWT fetch error:', jwtErr);
        }

        router.push('/dashboard');
        return true;
    };

    const checkExistingSession = async () => {
        try {
            const response = await fetch('/sessions/whoami', {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            if (!response.ok) {
                return false;
            }

            const data = await response.json().catch(() => ({}));
            const identity = data?.identity;

            if (!identity) {
                return false;
            }

            await handleAuthenticatedIdentity(identity);
            return true;
        } catch (err) {
            console.error('Session check error:', err);
            return false;
        }
    };

    useEffect(() => {
        checkExistingSession();

        const googleLoginError = sessionStorage.getItem(GOOGLE_LOGIN_ERROR_KEY);
        if (googleLoginError) {
            setFormError(googleLoginError);
            sessionStorage.removeItem(GOOGLE_LOGIN_ERROR_KEY);
        }
    }, []);

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
                await checkExistingSession();
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

    const handleGoogleLogin = async () => {
        setFormError('');

        if (!agreementChecked) {
            setAgreementError('Please agree to the Platform Agreement and Privacy Policy before continuing.');
            return;
        }

        setIsLoading(true);

        try {
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
                    await checkExistingSession();
                    return;
                }

                throw new Error(getErrorMessage(data, 'Failed to initialize Google login flow'));
            }

            const uiAction = data?.ui?.action;
            const uiMethod = (data?.ui?.method || 'POST').toUpperCase();
            const uiNodes = data?.ui?.nodes || [];

            if (!uiAction) {
                throw new Error('Login action not found in Kratos flow');
            }

            const googleProviderNode = uiNodes.find(
                (node) =>
                    node?.group === 'oidc' &&
                    node?.attributes?.name === 'provider' &&
                    node?.attributes?.value === 'google'
            );

            if (!googleProviderNode) {
                throw new Error('Google provider is not available in the Kratos login flow');
            }

            const form = document.createElement('form');
            form.method = uiMethod;
            form.action = uiAction;
            form.style.display = 'none';

            uiNodes.forEach((node) => {
                const attributes = node?.attributes || {};
                if (
                    attributes?.type === 'hidden' &&
                    typeof attributes?.name === 'string' &&
                    typeof attributes?.value !== 'undefined'
                ) {
                    appendHiddenInput(form, attributes.name, String(attributes.value));
                }
            });

            if (!form.querySelector('input[name="method"]')) {
                appendHiddenInput(form, 'method', 'oidc');
            }

            appendHiddenInput(form, 'provider', 'google');

            sessionStorage.removeItem(GOOGLE_LOGIN_ERROR_KEY);
            sessionStorage.setItem(GOOGLE_LOGIN_ATTEMPT_KEY, String(Date.now()));

            document.body.appendChild(form);
            form.submit();
        } catch (err) {
            console.error('Google Login Error:', err);
            setFormError(err.message || 'Failed to continue with Google login');
            setIsLoading(false);
        }
    };

    const handleLogin = async (e) => {
        e.preventDefault();
        setFormError('');

        if (!agreementChecked) {
            setAgreementError('Please agree to the Platform Agreement and Privacy Policy before continuing.');
            return;
        }

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
                await handleAuthenticatedIdentity(data?.session?.identity);
                return;
            }

            if (isAlreadyLoggedInError(data)) {
                await checkExistingSession();
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
            console.error('Login Error:', err);
            setFormError(err.message || 'An unexpected error occurred.');
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
                    <h2 className="login-header-title">Sign In</h2>
                    <p className="login-header-subtitle">Access your account by entering your login details below.</p>

                    {formError && (
                        <div style={{ color: 'red', marginBottom: '15px', padding: '10px', background: '#ffeeee', borderRadius: '4px' }}>
                            {formError}
                        </div>
                    )}

                    <div className="login-auth-mode-switch" role="tablist" aria-label="Choose sign in method">
                        <button
                            type="button"
                            role="tab"
                            aria-selected={openSection === 'social'}
                            className={`login-auth-mode-btn ${openSection === 'social' ? 'is-active' : ''}`}
                            onClick={() => setOpenSection('social')}
                        >
                            Sign in with Social Id
                        </button>
                        <button
                            type="button"
                            role="tab"
                            aria-selected={openSection === 'local'}
                            className={`login-auth-mode-btn ${openSection === 'local' ? 'is-active' : ''}`}
                            onClick={() => setOpenSection('local')}
                        >
                            Sign in Locally
                        </button>
                    </div>

                    <div className="login-auth-carousel" aria-live="polite">
                        <div className={`login-auth-track ${openSection === 'local' ? 'show-local' : 'show-social'}`}>
                            <section className={`login-auth-slide is-social ${openSection === 'social' ? 'is-active' : ''}`} aria-hidden={openSection !== 'social'}>
                                <div className="login-accordion-body">
                                    <button
                                        type="button"
                                        className="login-social-btn login-google-btn"
                                        disabled={isLoading}
                                        onClick={handleGoogleLogin}
                                    >
                                        <span className="login-social-icon login-google-icon">G</span>
                                        <span>{isLoading ? 'Please wait...' : 'Continue with Google'}</span>
                                    </button>

                                    <div className="login-disabled-auth-wrap" title="Feature not available now but will be there shortly.">
                                        <button type="button" className="login-social-btn login-azure-btn" disabled>
                                            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 21 21" className="login-social-icon login-azure-icon" aria-hidden="true">
                                                <rect x="1" y="1" width="9" height="9" fill="#3c4043" />
                                                <rect x="11" y="1" width="9" height="9" fill="#3c4043" />
                                                <rect x="1" y="11" width="9" height="9" fill="#3c4043" />
                                                <rect x="11" y="11" width="9" height="9" fill="#3c4043" />
                                            </svg>
                                            <span>Continue with Microsoft</span>
                                        </button>
                                    </div>
                                </div>
                            </section>

                            <section className={`login-auth-slide is-local ${openSection === 'local' ? 'is-active' : ''}`} aria-hidden={openSection !== 'local'}>
                                <form id="local-login-form" className="login-accordion-body login-auth-form" onSubmit={handleLogin}>
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
                                                type={showPassword ? 'text' : 'password'}
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
                                                title={showPassword ? 'Hide password' : 'Show password'}
                                                style={{ position: 'absolute', right: '10px', background: 'transparent', border: 'none', cursor: 'pointer', color: '#6b7280', display: 'flex', alignItems: 'center' }}
                                            >
                                                {showPassword ? <Eye size={20} /> : <EyeOff size={20} />}
                                            </button>
                                        </div>
                                    </div>
                                </form>
                            </section>
                        </div>
                    </div>

                    <div className="login-form-divider" />

                    <div className="legal-consent-row">
                        <input
                            id="login-agreement"
                            type="checkbox"
                            className="legal-consent-checkbox"
                            checked={agreementChecked}
                            onChange={(e) => {
                                setAgreementChecked(e.target.checked);
                                if (e.target.checked) setAgreementError('');
                            }}
                        />
                        <label htmlFor="login-agreement" className="legal-consent-label-copy">
                            I agree to the{' '}
                            <Link href="/terms" target="_blank" className="legal-consent-link-button">
                                Platform Agreement
                            </Link>{' '}
                            &{' '}
                            <Link href="/privacy" target="_blank" className="legal-consent-link-button">
                                Privacy Policy
                            </Link>
                        </label>
                        {agreementError && <p className="legal-consent-error">{agreementError}</p>}
                    </div>

                    <button
                        type="submit"
                        form="local-login-form"
                        className="login-submit-btn"
                        disabled={openSection !== 'local' || isLoading || !email.trim() || !password.trim() || !agreementChecked}
                    >
                        {isLoading ? 'Signing In...' : 'Sign In'}
                    </button>

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