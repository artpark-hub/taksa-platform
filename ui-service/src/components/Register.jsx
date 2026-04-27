'use client';

import React, { useState, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { Eye, EyeOff } from 'lucide-react'; 
import LegalDocumentModal from './LegalDocumentModal';

const GOOGLE_REGISTER_CONTEXT_KEY = 'taksa_google_register_context';
const GOOGLE_LOGIN_ATTEMPT_KEY = 'taksa_google_login_attempt';
const GOOGLE_LOGIN_ERROR_KEY = 'taksa_google_login_error';

const Register = () => {
    const router = useRouter();
    const searchParams = useSearchParams();
    const flowId = searchParams?.get('flow');

    const [formData, setFormData] = useState({
        firstName: '', lastName: '', email: '', orgName: '', password: '', confirmPassword: ''
    });

    const [showPassword, setShowPassword] = useState(false);
    const [showConfirmPassword, setShowConfirmPassword] = useState(false);

    const [errors, setErrors] = useState({});
    const [isLoading, setIsLoading] = useState(false);
    const [formError, setFormError] = useState('');
    const [agreementChecked, setAgreementChecked] = useState(false);
    const [agreementError, setAgreementError] = useState('');
    const [activeLegalDocument, setActiveLegalDocument] = useState(null);
    const [openSection, setOpenSection] = useState('social');
    const [googleModalOpen, setGoogleModalOpen] = useState(false);
    const [googleOrgName, setGoogleOrgName] = useState('');
    const [googleOrgError, setGoogleOrgError] = useState('');

    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

    const getErrorMessage = (data, fallback) => {
        return (
            data?.ui?.messages?.[0]?.text ||
            data?.error?.message ||
            data?.message ||
            fallback
        );
    };

    const appendHiddenInput = (form, name, value) => {
        const input = document.createElement('input');
        input.type = 'hidden';
        input.name = name;
        input.value = value;
        form.appendChild(input);
    };

    const extractCsrfToken = (flow) => {
        const nodes = flow?.ui?.nodes || [];
        const csrfNode = nodes.find((node) => node?.attributes?.name === 'csrf_token');
        return csrfNode?.attributes?.value || '';
    };

    const storeUserInLocalStorage = (identity) => {
        const traits = identity?.traits || {};

        localStorage.setItem('taksa_user', JSON.stringify({
            identity_id: identity.id,
            email: traits.email || '',
            first_name: traits.name?.first || '',
            last_name: traits.name?.last || '',
            role: traits.role || '',
            organization_name: traits.organization_name || '',
            organization_id: traits.organization_id || '',
            identityId: identity.id,
            firstName: traits.name?.first || '',
            lastName: traits.name?.last || '',
            organizationName: traits.organization_name || '',
            organizationId: traits.organization_id || ''
        }));
    };

    const setSecurePendingGoogleRegistration = async (pendingContext) => {
        const response = await fetch('/api/auth/google-register-context', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
            credentials: 'include',
            body: JSON.stringify(pendingContext)
        });

        return response.ok;
    };

    const getSecurePendingGoogleRegistration = async () => {
        const response = await fetch('/api/auth/google-register-context', {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include'
        });

        if (!response.ok) {
            return null;
        }

        const data = await response.json().catch(() => ({}));
        return data?.pending || null;
    };

    const clearSecurePendingGoogleRegistration = async () => {
        try {
            await fetch('/api/auth/google-register-context', {
                method: 'DELETE',
                headers: { Accept: 'application/json' },
                credentials: 'include'
            });
        } catch {
        }
    };

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

    const fetchOrganizationId = async () => {
        const orgIdRes = await fetch('/api/v1/um/generate_organization_id', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
            credentials: 'include',
            body: JSON.stringify({})
        });

        const orgIdData = await orgIdRes.json().catch(() => ({}));

        if (!orgIdRes.ok) {
            throw new Error(
                orgIdData?.message ||
                orgIdData?.error?.message ||
                'Failed to generate organization id'
            );
        }

        const organizationId =
            orgIdData?.organization_id ||
            orgIdData?.organizationId ||
            orgIdData?.data?.organization_id ||
            orgIdData?.data?.organizationId;

        if (!organizationId) throw new Error('Organization id not found in response');

        return organizationId;
    };

    const storePendingGoogleRegistration = async ({ organizationName, organizationId }) => {
        const pendingContext = {
            organization_name: organizationName,
            organization_id: organizationId,
            role: 'master',
            source: 'google',
            created_at: Date.now()
        };

        try {
            await setSecurePendingGoogleRegistration(pendingContext);
        } catch {
        }
        sessionStorage.setItem(GOOGLE_REGISTER_CONTEXT_KEY, JSON.stringify(pendingContext));
    };

    const startGoogleOidcRegistration = async () => {
        const initRes = await fetch('/self-service/registration/browser', {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include',
            redirect: 'follow'
        });

        const initData = await initRes.json().catch(() => ({}));

        if (!initRes.ok) {
            throw new Error(getErrorMessage(initData, 'Failed to initialize Google registration flow'));
        }

        const uiAction = initData?.ui?.action;
        const uiMethod = (initData?.ui?.method || 'POST').toUpperCase();
        const uiNodes = initData?.ui?.nodes || [];

        if (!uiAction) {
            throw new Error('Registration action not found in Kratos flow');
        }

        const googleProviderNode = uiNodes.find(
            node =>
                node?.group === 'oidc' &&
                node?.attributes?.name === 'provider' &&
                node?.attributes?.value === 'google'
        );

        if (!googleProviderNode) {
            throw new Error('Google provider is not available in the Kratos registration flow');
        }

        const form = document.createElement('form');
        form.method = uiMethod;
        form.action = uiAction;
        form.style.display = 'none';

        uiNodes.forEach(node => {
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

        document.body.appendChild(form);
        form.submit();
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

    const handleGoogleContinue = async () => {
        setFormError('');
        setGoogleOrgError('');

        if (!agreementChecked) {
            setAgreementError('Please agree to the Platform Agreement and Privacy Policy before continuing.');
            return;
        }

        const trimmedOrgName = googleOrgName.trim();

        if (!trimmedOrgName) {
            setGoogleOrgError('Organisation name is required.');
            return;
        }

        setIsLoading(true);

        try {
            const organizationId = await fetchOrganizationId();

            await storePendingGoogleRegistration({
                organizationName: trimmedOrgName,
                organizationId
            });

            await startGoogleOidcRegistration();
        } catch (err) {
            console.error('Google Registration Error:', err);
            setFormError(err.message || 'Failed to continue with Google registration');
            setIsLoading(false);
        }
    };

    useEffect(() => {
        if (!flowId) return;

        let cancelled = false;

        const completeOidcRegistration = async () => {
            const loginAttemptTs = Number(sessionStorage.getItem(GOOGLE_LOGIN_ATTEMPT_KEY) || 0);
            const isRecentGoogleLoginAttempt = loginAttemptTs > 0 && (Date.now() - loginAttemptTs) < 10 * 60 * 1000;

            let pending = null;

            try {
                pending = await getSecurePendingGoogleRegistration();
            } catch {
                pending = null;
            }

            if (!pending) {
                const pendingRaw = sessionStorage.getItem(GOOGLE_REGISTER_CONTEXT_KEY);
                if (pendingRaw) {
                    try {
                        pending = JSON.parse(pendingRaw);
                    } catch {
                        pending = null;
                    }
                }
            }

            if (!pending && isRecentGoogleLoginAttempt) {
                sessionStorage.removeItem(GOOGLE_LOGIN_ATTEMPT_KEY);
                sessionStorage.setItem(
                    GOOGLE_LOGIN_ERROR_KEY,
                    'Google sign-in could not be completed. This account is either new or not yet linked—please register first, or sign in with email/password and link Google later.'
                );
                router.replace('/');
                return;
            }

            if (!pending) {
                if (cancelled) return;
                setFormError('Google registration context expired or was not found. Please start Google registration again.');
                setOpenSection('social');
                router.replace('/register');
                return;
            }

            if (!pending?.source || Date.now() - (pending.created_at || 0) > 10 * 60 * 1000) {
                await clearSecurePendingGoogleRegistration();
                sessionStorage.removeItem(GOOGLE_REGISTER_CONTEXT_KEY);
                if (cancelled) return;
                setFormError('Google registration context expired. Please start Google registration again.');
                setOpenSection('social');
                router.replace('/register');
                return;
            }

            if (cancelled) return;

            setIsLoading(true);
            setFormError('');

            try {
                const flowRes = await fetch(`/self-service/registration/flows?id=${flowId}`, {
                    method: 'GET',
                    headers: { Accept: 'application/json' },
                    credentials: 'include'
                });

                const flowData = await flowRes.json().catch(() => ({}));

                if (!flowRes.ok) {
                    throw new Error(getErrorMessage(flowData, 'Failed to fetch registration flow'));
                }

                const actionUrl = flowData?.ui?.action;
                const actionMethod = (flowData?.ui?.method || 'POST').toUpperCase();
                const nodes = flowData?.ui?.nodes || [];

                if (!actionUrl) {
                    throw new Error('Registration action URL not found in flow');
                }

                const form = document.createElement('form');
                form.method = actionMethod;
                form.action = actionUrl;
                form.style.display = 'none';

                const addedNames = new Set();

                nodes.forEach((node) => {
                    const attrs = node?.attributes || {};
                    if (
                        attrs?.type === 'hidden' &&
                        typeof attrs?.name === 'string' &&
                        typeof attrs?.value !== 'undefined'
                    ) {
                        appendHiddenInput(form, attrs.name, String(attrs.value));
                        addedNames.add(attrs.name);
                    }
                });

                const nodeValue = (name) => {
                    const node = nodes.find((n) => n?.attributes?.name === name);
                    return node?.attributes?.value ?? '';
                };

                const traitFields = {
                    'traits.email': nodeValue('traits.email'),
                    'traits.name.first': nodeValue('traits.name.first'),
                    'traits.name.last': nodeValue('traits.name.last'),
                    'traits.organization_name': pending.organization_name,
                    'traits.organization_id': pending.organization_id,
                    'traits.role': pending.role || 'master',
                };

                Object.entries(traitFields).forEach(([name, value]) => {
                    if (!addedNames.has(name)) {
                        appendHiddenInput(form, name, String(value));
                        addedNames.add(name);
                    }
                });

                if (!addedNames.has('method')) {
                    appendHiddenInput(form, 'method', 'oidc');
                    addedNames.add('method');
                }

                if (!addedNames.has('provider')) {
                    appendHiddenInput(form, 'provider', 'google');
                    addedNames.add('provider');
                }

                await clearSecurePendingGoogleRegistration();
                sessionStorage.removeItem(GOOGLE_REGISTER_CONTEXT_KEY);
                sessionStorage.removeItem(GOOGLE_LOGIN_ATTEMPT_KEY);

                if (cancelled) return;

                document.body.appendChild(form);
                form.submit();
            } catch (err) {
                console.error('OIDC completion error:', err);
                await clearSecurePendingGoogleRegistration();
                sessionStorage.removeItem(GOOGLE_REGISTER_CONTEXT_KEY);
                sessionStorage.removeItem(GOOGLE_LOGIN_ATTEMPT_KEY);
                if (!cancelled) {
                    setFormError(err.message || 'Google registration failed. Please try again.');
                    setIsLoading(false);
                }
            }
        };

        completeOidcRegistration();

        return () => {
            cancelled = true;
        };
    }, [flowId, router]);

    const registerWithKratos = async () => {
        const initRes = await fetch('/self-service/registration/browser', {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include',
            redirect: 'follow'
        });

        const initData = await initRes.json().catch(() => ({}));

        if (!initRes.ok) {
            throw new Error(getErrorMessage(initData, 'Failed to initialize registration flow'));
        }

        const initFlowId = initData?.id;
        if (!initFlowId) throw new Error('Registration flow id not found');

        const flowRes = await fetch(`/self-service/registration/flows?id=${initFlowId}`, {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include'
        });

        const flowData = await flowRes.json().catch(() => ({}));

        if (!flowRes.ok) {
            throw new Error(getErrorMessage(flowData, 'Failed to fetch registration flow'));
        }

        const organizationId = await fetchOrganizationId();

        const registerRes = await fetch(`/self-service/registration?flow=${initFlowId}`, {
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
                    organization_name: formData.orgName,
                    organization_id: organizationId
                },
                csrf_token: extractCsrfToken(flowData)
            })
        });

        const registerData = await registerRes.json().catch(() => ({}));

        if (!registerRes.ok) {
            throw new Error(getErrorMessage(registerData, 'Registration failed'));
        }

        return registerData;
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

        if (!agreementChecked) {
            setAgreementError('Please agree to the Platform Agreement and Privacy Policy before continuing.');
            return;
        }

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
            if (identity) {
                storeUserInLocalStorage(identity);
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
                    <h2 className="register-header-title">Register</h2>
                    <p className="register-header-subtitle">Create your account by entering your registration details below.</p>

                    {formError && (
                        <div style={{ color: 'red', marginBottom: '15px', padding: '10px', background: '#ffeeee', borderRadius: '4px' }}>
                            {formError}
                        </div>
                    )}

                    <div className="register-auth-mode-switch" role="tablist" aria-label="Choose registration method">
                        <button
                            type="button"
                            role="tab"
                            aria-selected={openSection === 'social'}
                            className={`register-auth-mode-btn ${openSection === 'social' ? 'is-active' : ''}`}
                            onClick={() => setOpenSection('social')}
                        >
                            Register with Social Id
                        </button>
                        <button
                            type="button"
                            role="tab"
                            aria-selected={openSection === 'local'}
                            className={`register-auth-mode-btn ${openSection === 'local' ? 'is-active' : ''}`}
                            onClick={() => setOpenSection('local')}
                        >
                            Register Locally
                        </button>
                    </div>

                    <div className="register-auth-carousel" aria-live="polite">
                        <div className={`register-auth-track ${openSection === 'local' ? 'show-local' : 'show-social'}`}>
                            <section className={`register-auth-slide is-social ${openSection === 'social' ? 'is-active' : ''}`} aria-hidden={openSection !== 'social'}>
                                <div className="register-accordion-body">
                                    <button
                                        type="button"
                                        className="register-social-btn register-google-btn"
                                        disabled={isLoading}
                                        onClick={() => {
                                            setGoogleOrgName('');
                                            setGoogleOrgError('');
                                            setGoogleModalOpen(true);
                                        }}
                                    >
                                        <span className="register-social-icon register-google-icon">G</span>
                                        <span>{isLoading ? 'Please wait...' : 'Continue with Google'}</span>
                                    </button>

                                    <div className="register-disabled-auth-wrap" title="Feature not available now but will be there shortly.">
                                        <button type="button" className="register-social-btn register-azure-btn" disabled>
                                            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 21 21" className="register-social-icon register-azure-icon" aria-hidden="true">
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

                            <section className={`register-auth-slide is-local ${openSection === 'local' ? 'is-active' : ''}`} aria-hidden={openSection !== 'local'}>
                                <form id="local-register-form" className="register-auth-form" onSubmit={handleSubmit}>
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
                                            type="email"
                                            name="email"
                                            placeholder="user@yourcompany.com"
                                            value={formData.email}
                                            onChange={handleChange}
                                            onBlur={handleBlur}
                                            className={errors.email ? 'register-input-error-border' : ''}
                                            disabled={isLoading}
                                        />
                                        {errors.email && <span className="register-error-message">{errors.email}</span>}
                                    </div>

                                    <div className="register-input-group">
                                        <label>Password *</label>
                                        {errors.password && (
                                            <span
                                                className="register-error-message"
                                                style={{ color: 'red', fontSize: '0.8rem', marginBottom: '4px', display: 'block' }}
                                            >
                                                {errors.password}
                                            </span>
                                        )}
                                        <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
                                            <input
                                                type={showPassword ? "text" : "password"}
                                                name="password"
                                                value={formData.password}
                                                onChange={handleChange}
                                                disabled={isLoading}
                                                style={{ width: '100%', paddingRight: '45px' }}
                                            />
                                            <button
                                                type="button"
                                                onClick={() => setShowPassword(!showPassword)}
                                                style={{ position: 'absolute', right: '10px', background: 'transparent', border: 'none', cursor: 'pointer', color: '#6b7280', display: 'flex', alignItems: 'center' }}
                                            >
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
                                            <button
                                                type="button"
                                                onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                                                style={{ position: 'absolute', right: '10px', background: 'transparent', border: 'none', cursor: 'pointer', color: '#6b7280', display: 'flex', alignItems: 'center' }}
                                            >
                                                {showConfirmPassword ? <Eye size={20} /> : <EyeOff size={20} />}
                                            </button>
                                        </div>
                                        {errors.confirmPassword && (
                                            <span
                                                className="register-error-message"
                                                style={{ color: 'red', fontSize: '0.8rem', marginTop: '4px', display: 'block' }}
                                            >
                                                {errors.confirmPassword}
                                            </span>
                                        )}
                                    </div>
                                </form>
                            </section>
                        </div>
                    </div>

                    <div className="register-auth-form">
                        <div className="register-form-divider" />

                        <div className="legal-consent-card">
                            <div className="legal-consent-row">
                                <input
                                    id="register-agreement"
                                    type="checkbox"
                                    className="legal-consent-checkbox"
                                    checked={agreementChecked}
                                    onChange={(e) => {
                                        setAgreementChecked(e.target.checked);
                                        if (e.target.checked) setAgreementError('');
                                    }}
                                />
                                <div className="legal-consent-copy">
                                    <p className="legal-consent-title">
                                        <label htmlFor="register-agreement" className="legal-consent-label-copy">I agree to the </label>
                                        <button type="button" className="legal-consent-link-button" onClick={() => setActiveLegalDocument('terms')}>
                                            Platform Agreement
                                        </button>{' '}
                                        &{' '}
                                        <button type="button" className="legal-consent-link-button" onClick={() => setActiveLegalDocument('privacy')}>
                                            Privacy Policy
                                        </button>
                                    </p>
                                    {agreementError && <p className="legal-consent-error">{agreementError}</p>}
                                </div>
                            </div>
                        </div>

                        <button
                            type="submit"
                            form="local-register-form"
                            className="register-submit-btn"
                            disabled={isLoading || !isFormValid || !agreementChecked || openSection !== 'local'}
                        >
                            {isLoading ? "Creating Account..." : "Register"}
                        </button>
                    </div>

                    <div className="register-footer-section">
                        <span>Already have an account? <Link href="/">Sign in now</Link></span>
                    </div>
                </div>
            </div>

            <LegalDocumentModal
                documentKey={activeLegalDocument || 'terms'}
                open={Boolean(activeLegalDocument)}
                onClose={() => setActiveLegalDocument(null)}
            />

            {googleModalOpen && (
                <div className="reg-google-modal-overlay" onClick={() => !isLoading && setGoogleModalOpen(false)}>
                    <div
                        className="reg-google-modal"
                        onClick={(e) => e.stopPropagation()}
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="google-modal-title"
                    >
                        <h3 id="google-modal-title" className="reg-google-modal-title">Enter Organisation Name</h3>
                        <p className="reg-google-modal-subtitle">Please provide your organisation name before continuing with Google.</p>

                        <input
                            type="text"
                            className="reg-google-modal-input"
                            placeholder="Your organisation name"
                            value={googleOrgName}
                            onChange={(e) => {
                                setGoogleOrgName(e.target.value);
                                if (googleOrgError) setGoogleOrgError('');
                            }}
                            autoFocus
                            disabled={isLoading}
                        />

                        {googleOrgError && (
                            <p style={{ color: 'red', fontSize: '0.85rem', marginTop: '8px' }}>
                                {googleOrgError}
                            </p>
                        )}

                        <button
                            type="button"
                            className="reg-google-modal-btn"
                            disabled={isLoading || !googleOrgName.trim()}
                            onClick={handleGoogleContinue}
                        >
                            {isLoading ? 'Continuing...' : 'Click to continue'}
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Register;