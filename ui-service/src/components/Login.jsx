'use client';

import React, { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Eye, EyeOff } from 'lucide-react'; 
import LegalDocumentModal from './LegalDocumentModal';

const GOOGLE_LOGIN_ATTEMPT_KEY = 'taksa_google_login_attempt';
const GOOGLE_LOGIN_ERROR_KEY = 'taksa_google_login_error';

const Login = () => {
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [showPassword, setShowPassword] = useState(false);
    const [emailError, setEmailError] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [formError, setFormError] = useState('');
    const [openSection, setOpenSection] = useState('social');

    const [pendingIdentity, setPendingIdentity] = useState(null);
    const [googleModalOpen, setGoogleModalOpen] = useState(false);
    const [googleOrgName, setGoogleOrgName] = useState('');
    const [googleOrgError, setGoogleOrgError] = useState('');
    const [agreementChecked, setAgreementChecked] = useState(false);
    const [agreementError, setAgreementError] = useState('');
    const [activeLegalDocument, setActiveLegalDocument] = useState(null);

    const router = useRouter();

    const validateEmail = (value) => {
        const pattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        return pattern.test(value);
    };

    const clearStoredAuth = () => {
        localStorage.removeItem('taksa_user');
        localStorage.removeItem('taksa_jwt');
    };

    const clearGoogleLoginState = () => {
        sessionStorage.removeItem(GOOGLE_LOGIN_ATTEMPT_KEY);
        sessionStorage.removeItem(GOOGLE_LOGIN_ERROR_KEY);
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

    const isRecentGoogleLoginAttempt = () => {
        const loginAttemptTs = Number(sessionStorage.getItem(GOOGLE_LOGIN_ATTEMPT_KEY) || 0);
        return loginAttemptTs > 0 && Date.now() - loginAttemptTs < 10 * 60 * 1000;
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

    const fetchSessionIdentity = async () => {
        const response = await fetch('/sessions/whoami', {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include'
        });

        if (!response.ok) {
            return null;
        }

        const data = await response.json().catch(() => ({}));
        return data?.identity || null;
    };

    const logoutKratosSession = async () => {
        try {
            const logoutFlowRes = await fetch('/self-service/logout/browser', {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            const logoutFlowData = await logoutFlowRes.json().catch(() => ({}));
            const logoutUrl = logoutFlowData?.logout_url;

            if (logoutUrl) {
                await fetch(logoutUrl, {
                    method: 'GET',
                    credentials: 'include'
                });
            }
        } catch (err) {
            console.error('Logout error:', err);
        }
    };

    const clearExistingSessionAndAuth = async () => {
        await logoutKratosSession();
        clearStoredAuth();
        clearGoogleLoginState();
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

    const fetchOrganizationId = async () => {
        const orgIdRes = await fetch('/api/v1/um/generate_organization_id', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
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
            orgIdData?.tenant_id ||
            orgIdData?.tenantId ||
            orgIdData?.data?.tenant_id ||
            orgIdData?.data?.tenantId ||
            orgIdData?.organization_id ||
            orgIdData?.organizationId ||
            orgIdData?.data?.organization_id ||
            orgIdData?.data?.organizationId;

        if (!organizationId) {
            throw new Error('Organization id not found in response');
        }

        return organizationId;
    };

    const checkUserStatus = async (email) => {
        const normalizedEmail = String(email || '').trim().toLowerCase();
        if (!normalizedEmail) {
            return {
                message: ''
            };
        }

        const response = await fetch('/api/v1/um/check_user', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
            credentials: 'include',
            body: JSON.stringify({
                email: normalizedEmail
            })
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(
                data?.message ||
                data?.error?.message ||
                'Failed to check user'
            );
        }

        const role = String(
            data?.role ||
            data?.data?.role ||
            ''
        ).trim().toLowerCase();

        const organizationName = String(
            data?.organization_name ||
            data?.organizationName ||
            data?.data?.organization_name ||
            data?.data?.organizationName ||
            ''
        ).trim();

        const tenantId = String(
            data?.tenant_id ||
            data?.tenantId ||
            data?.data?.tenant_id ||
            data?.data?.tenantId ||
            ''
        ).trim();

        const message = String(data?.message || '').trim();

        return {
            role,
            organizationName,
            tenantId,
            message
        };
    };

    const updateIdentityTraits = async ({ identity, organizationName, tenantId, role }) => {
        const traits = identity?.traits || {};

        const initRes = await fetch('/self-service/settings/browser', {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include',
            redirect: 'follow'
        });

        const initData = await initRes.json().catch(() => ({}));

        if (!initRes.ok) {
            throw new Error(getErrorMessage(initData, 'Failed to initialize profile update flow'));
        }

        const flowId = initData?.id;

        if (!flowId) {
            throw new Error('Settings flow id not found');
        }

        const flowRes = await fetch(`/self-service/settings/flows?id=${encodeURIComponent(flowId)}`, {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include'
        });

        const flowData = await flowRes.json().catch(() => ({}));

        if (!flowRes.ok) {
            throw new Error(getErrorMessage(flowData, 'Failed to fetch profile update flow'));
        }

        const updatedTraits = {
            ...traits,
            email: traits.email || '',
            name: {
                first: traits.name?.first || '',
                last: traits.name?.last || ''
            },
            role: String(role || traits.role || 'master').trim().toLowerCase(),
            organization_name: organizationName,
            tenant_id: tenantId
        };

        const updateRes = await fetch(`/self-service/settings?flow=${encodeURIComponent(flowId)}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
            credentials: 'include',
            body: JSON.stringify({
                method: 'profile',
                csrf_token: extractCsrfToken(flowData),
                traits: updatedTraits
            })
        });

        const updateData = await updateRes.json().catch(() => ({}));

        if (!updateRes.ok) {
            const redirectTo =
                updateData?.redirect_browser_to ||
                updateData?.error?.redirect_browser_to ||
                updateData?.error?.details?.redirect_browser_to;

            const errorMessage =
                updateData?.error?.message ||
                updateData?.ui?.messages?.[0]?.text ||
                updateData?.message ||
                '';

            if (
                updateRes.status === 422 &&
                redirectTo &&
                errorMessage.toLowerCase().includes('browser location change required')
            ) {
                window.location.assign(redirectTo);
                return false;
            }

            throw new Error(getErrorMessage(updateData, 'Failed to update organisation details'));
        }

        return true;
    };

    const completeLocalLogin = async (identity) => {
        if (!identity) {
            throw new Error('Logged in user identity not found');
        }

        clearGoogleLoginState();

        storeUserInLocalStorage(identity);
        await fetchJwt();

        router.push('/dashboard');
    };

    const openOrganisationModal = (identity) => {
        clearStoredAuth();
        setPendingIdentity(identity);
        setGoogleOrgName('');
        setGoogleOrgError('');
        setAgreementChecked(false);
        setAgreementError('');
        setGoogleModalOpen(true);
        setIsLoading(false);
    };

    const completeGoogleLogin = async (identity) => {
        if (!identity) {
            throw new Error('Logged in user identity not found');
        }

        const traits = identity?.traits || {};
        let finalIdentity = identity;

        const userStatus = await checkUserStatus(traits?.email || '');
        const normalizedMessage = String(userStatus.message || '').toLowerCase();

        if (normalizedMessage === 'user is already registered.') {
            const refreshedIdentity = await fetchSessionIdentity();
            if (refreshedIdentity) {
                finalIdentity = refreshedIdentity;
            }
        } else if (userStatus.organizationName && userStatus.tenantId) {
            const updated = await updateIdentityTraits({
                identity,
                organizationName: userStatus.organizationName,
                tenantId: userStatus.tenantId,
                role: userStatus.role
            });

            if (!updated) {
                return false;
            }

            const refreshedIdentity = await fetchSessionIdentity();
            if (
                !refreshedIdentity ||
                !String(refreshedIdentity?.traits?.organization_name || '').trim() ||
                !String(refreshedIdentity?.traits?.tenant_id || '').trim()
            ) {
                throw new Error('Failed to sync matched organization details. Please try again.');
            }

            finalIdentity = refreshedIdentity;
        } else {
            openOrganisationModal(identity);
            return false;
        }

        clearGoogleLoginState();

        storeUserInLocalStorage(finalIdentity);
        await fetchJwt();

        router.push('/dashboard');
        return true;
    };

    const checkGoogleSessionAfterRedirect = async () => {
        if (!isRecentGoogleLoginAttempt()) {
            return false;
        }

        try {
            const identity = await fetchSessionIdentity();

            if (!identity) {
                return false;
            }

            await completeGoogleLogin(identity);
            return true;
        } catch (err) {
            console.error('Google session check error:', err);
            setFormError(err.message || 'Failed to complete Google sign in');
            return false;
        }
    };

    useEffect(() => {
        checkGoogleSessionAfterRedirect();

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
                await clearExistingSessionAndAuth();
                throw new Error('An existing session was found and cleared. Please click Sign In again.');
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
                    await clearExistingSessionAndAuth();
                    throw new Error('An existing session was found and cleared. Please click Continue with Google again.');
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

    const deleteIncompleteOidcUser = async (identity) => {
        const identityId = identity?.id;

        if (!identityId) {
            return;
        }

        const latestIdentity = await fetchSessionIdentity();

        if (!latestIdentity || latestIdentity.id !== identityId) {
            throw new Error('Unable to verify incomplete Google account before deletion.');
        }

        if (
            String(latestIdentity?.traits?.organization_name || '').trim() ||
            String(latestIdentity?.traits?.tenant_id || '').trim()
        ) {
            throw new Error('Account deletion skipped because organisation details already exist.');
        }

        const response = await fetch(`/api/v1/um/delete_incomplete_oidc_user/${encodeURIComponent(identityId)}`, {
            method: 'DELETE',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include'
        });

        if (!response.ok && response.status !== 404) {
            const data = await response.json().catch(() => ({}));

            throw new Error(
                data?.message ||
                data?.error?.message ||
                'Failed to delete incomplete Google account'
            );
        }
    };

    const handleCloseGoogleModal = async () => {
        if (isLoading) return;

        const identityToDelete = pendingIdentity;

        setIsLoading(true);
        setFormError('');

        try {
            await deleteIncompleteOidcUser(identityToDelete);

            setGoogleModalOpen(false);
            setPendingIdentity(null);
            setGoogleOrgName('');
            setGoogleOrgError('');
            setAgreementChecked(false);
            setAgreementError('');

            await clearExistingSessionAndAuth();

            setFormError('Google setup was cancelled. The incomplete account was removed. Please sign in again.');
        } catch (err) {
            console.error('Incomplete Google account cleanup error:', err);

            setFormError(
                err.message ||
                'Unable to cancel setup safely. Please try again.'
            );
        } finally {
            setIsLoading(false);
        }
    };

    const handleCompleteOrganisationDetails = async () => {
        setFormError('');
        setGoogleOrgError('');
        setAgreementError('');

        if (!agreementChecked) {
            setAgreementError('Please agree to the Platform Agreement and Privacy Policy before continuing.');
            return;
        }

        const trimmedOrgName = googleOrgName.trim();

        if (!trimmedOrgName) {
            setGoogleOrgError('Organisation name is required.');
            return;
        }

        if (!pendingIdentity) {
            setFormError('Logged in user identity not found. Please sign in again.');
            return;
        }

        setIsLoading(true);

        try {
            const tenantId = await fetchOrganizationId();

            const updated = await updateIdentityTraits({
                identity: pendingIdentity,
                organizationName: trimmedOrgName,
                tenantId
            });

            if (!updated) return;

            const updatedIdentity = await fetchSessionIdentity();

            if (!updatedIdentity) {
                throw new Error('Unable to verify updated session. Please sign in again.');
            }

            if (
                !String(updatedIdentity?.traits?.organization_name || '').trim() ||
                !String(updatedIdentity?.traits?.tenant_id || '').trim()
            ) {
                throw new Error('Organisation details were not saved. Please try again.');
            }

            setGoogleModalOpen(false);
            setPendingIdentity(null);

            await completeGoogleLogin(updatedIdentity);
        } catch (err) {
            console.error('Organisation completion error:', err);
            setFormError(err.message || 'Failed to complete organisation details');
            setIsLoading(false);
        }
    };

    const handleLogin = async (e) => {
        e.preventDefault();
        setFormError('');
        clearGoogleLoginState();

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
                await completeLocalLogin(data?.session?.identity);
                return;
            }

            if (isAlreadyLoggedInError(data)) {
                await clearExistingSessionAndAuth();
                setFormError('An existing session was found and cleared. Please click Sign In again.');
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

                    {openSection === 'local' && (
                        <button
                            type="submit"
                            form="local-login-form"
                            className="login-submit-btn"
                            disabled={isLoading || !email.trim() || !password.trim()}
                        >
                            {isLoading ? 'Signing In...' : 'Sign In'}
                        </button>
                    )}

                    <div className={`login-footer-section${openSection === 'local' ? ' has-top-margin' : ''}`}>
                        <span>Don't have a local account? <Link href="/register">Sign up now</Link></span>
                        <Link href="/recovery" style={{ fontSize: '0.85rem' }}>Forgot Password?</Link>
                    </div>
                </div>
            </div>

            <LegalDocumentModal
                documentKey={activeLegalDocument || 'terms'}
                open={Boolean(activeLegalDocument)}
                onClose={() => setActiveLegalDocument(null)}
            />

            {googleModalOpen && (
                <div className="login-google-modal-overlay">
                    <div
                        className="login-google-modal"
                        onClick={(e) => e.stopPropagation()}
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="login-google-modal-title"
                    >
                        <button
                            type="button"
                            className="login-google-modal-close"
                            onClick={handleCloseGoogleModal}
                            aria-label="Close organisation setup"
                            disabled={isLoading}
                        >
                            ×
                        </button>

                        <h3 id="login-google-modal-title" className="login-google-modal-title">Enter Organisation Name</h3>
                        <p className="login-google-modal-subtitle">
                            Please provide your organisation name to complete your Taksa account setup.
                        </p>

                        <div className="login-google-modal-consent-card">
                            <div className="login-google-modal-consent-row">
                                <input
                                    id="login-google-agreement"
                                    type="checkbox"
                                    className="login-google-modal-consent-checkbox"
                                    checked={agreementChecked}
                                    onChange={(e) => {
                                        setAgreementChecked(e.target.checked);
                                        if (e.target.checked) setAgreementError('');
                                    }}
                                    disabled={isLoading}
                                />

                                <div className="login-google-modal-consent-copy">
                                    <p className="login-google-modal-consent-title">
                                        <label htmlFor="login-google-agreement" className="login-google-modal-consent-label-copy">I agree to the </label>
                                        <button type="button" className="login-google-modal-link-button" onClick={() => setActiveLegalDocument('terms')}>
                                            Platform Agreement
                                        </button>{' '}
                                        &{' '}
                                        <button type="button" className="login-google-modal-link-button" onClick={() => setActiveLegalDocument('privacy')}>
                                            Privacy Policy
                                        </button>
                                    </p>

                                    {agreementError && (
                                        <p className="login-google-modal-consent-error">
                                            {agreementError}
                                        </p>
                                    )}
                                </div>
                            </div>
                        </div>

                        <input
                            type="text"
                            className={`login-google-modal-input ${googleOrgError ? 'login-google-modal-input-error' : ''}`}
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
                            <p className="login-google-modal-field-error">
                                {googleOrgError}
                            </p>
                        )}

                        <button
                            type="button"
                            className="login-google-modal-btn"
                            disabled={isLoading || !agreementChecked || !googleOrgName.trim()}
                            onClick={handleCompleteOrganisationDetails}
                        >
                            {isLoading ? 'Completing setup...' : 'Click to continue'}
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Login;