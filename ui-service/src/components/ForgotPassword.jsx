'use client';

import React, { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import './ForgotPassword.css';

const ForgotPassword = () => {
    const [email, setEmail] = useState('');
    const [emailError, setEmailError] = useState('');
    const [recoveryFlowId, setRecoveryFlowId] = useState('');
    const [recoveryCsrfToken, setRecoveryCsrfToken] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [isInitializing, setIsInitializing] = useState(true);
    const [formError, setFormError] = useState('');

    const didInitRef = useRef(false);

    const validateEmail = (value) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);

    const getErrorMessage = (data, fallback) =>
        data?.ui?.messages?.[0]?.text ||
        data?.error?.message ||
        data?.message ||
        fallback;

    const extractCsrfToken = (flow) => {
        const nodes = flow?.ui?.nodes || [];
        const csrfNode = nodes.find(
            (node) => node?.attributes?.name === 'csrf_token'
        );
        return csrfNode?.attributes?.value || '';
    };

    const replaceUrlWithFlow = (flowId) => {
        if (!flowId || typeof window === 'undefined') return;
        const url = new URL(window.location.href);
        url.searchParams.set('flow', flowId);
        window.history.replaceState({}, '', `${url.pathname}${url.search}`);
    };

    const clearFlowFromUrl = () => {
        if (typeof window === 'undefined') return;
        const url = new URL(window.location.href);
        url.searchParams.delete('flow');
        window.history.replaceState({}, '', `${url.pathname}${url.search}`);
    };

    const hasActiveSession = async () => {
        try {
            const response = await fetch('/sessions/whoami', {
                method: 'GET',
                headers: { Accept: 'application/json' },
                credentials: 'include',
                cache: 'no-store'
            });
            return response.ok;
        } catch {
            return false;
        }
    };

    const logoutCurrentSession = async () => {
        try {
            localStorage.removeItem('taksa_session_token');
            localStorage.removeItem('taksa_jwt');
            localStorage.removeItem('taksa_user');

            const res = await fetch('/self-service/logout/browser', {
                method: 'GET',
                credentials: 'include',
                headers: { Accept: 'application/json' },
                cache: 'no-store'
            });

            const data = await res.json().catch(() => ({}));

            if (res.ok && data?.logout_url) {
                const kratosLogoutUrl = new URL(data.logout_url);
                const finalLogoutUrl = `${window.location.origin}${kratosLogoutUrl.pathname}${kratosLogoutUrl.search}`;

                await fetch(finalLogoutUrl, {
                    method: 'GET',
                    credentials: 'include',
                    cache: 'no-store',
                    redirect: 'follow'
                });
            }
        } catch (err) {
            console.error('Logout before recovery failed:', err);
        }
    };

    const createFreshRecoveryFlow = async () => {
        const response = await fetch('/self-service/recovery/browser', {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include',
            cache: 'no-store'
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(getErrorMessage(data, 'Failed to start recovery flow'));
        }

        const flowId = data?.id;
        if (!flowId) {
            throw new Error('Recovery flow id was not returned');
        }

        setRecoveryFlowId(flowId);
        setRecoveryCsrfToken(extractCsrfToken(data));
        replaceUrlWithFlow(flowId);

        return data;
    };

    useEffect(() => {
        if (didInitRef.current) return;
        didInitRef.current = true;

        const initialize = async () => {
            setIsInitializing(true);
            setFormError('');

            try {
                const active = await hasActiveSession();
                if (active) {
                    await logoutCurrentSession();
                }

                clearFlowFromUrl();
                await createFreshRecoveryFlow();
            } catch (err) {
                console.error('Recovery init failed:', err);
                setRecoveryFlowId('');
                setRecoveryCsrfToken('');
                setFormError(err.message || 'Unable to initialize recovery. Please try again.');
            } finally {
                setIsInitializing(false);
            }
        };

        initialize();
    }, []);

    const handleSendCode = async (e) => {
        e.preventDefault();
        setFormError('');

        if (!email || !validateEmail(email)) {
            setEmailError('Please enter a valid email address');
            return;
        }

        if (!recoveryFlowId) {
            setFormError('Recovery session is missing. Please refresh and try again.');
            return;
        }

        setEmailError('');
        setIsLoading(true);

        try {
            const response = await fetch(`/self-service/recovery?flow=${recoveryFlowId}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    method: 'code',
                    email: email.trim(),
                    csrf_token: recoveryCsrfToken
                })
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to send recovery code'));
            }

            const nextFlowId = data?.id || recoveryFlowId;
            const nextCsrfToken = extractCsrfToken(data) || recoveryCsrfToken;

            setRecoveryFlowId(nextFlowId);
            setRecoveryCsrfToken(nextCsrfToken);

            sessionStorage.setItem('recovery_flow_id', nextFlowId);
            sessionStorage.setItem('recovery_csrf_token', nextCsrfToken);
            sessionStorage.setItem('recovery_email', email.trim());

            replaceUrlWithFlow(nextFlowId);

            window.location.href = `/reset-password?flow=${encodeURIComponent(nextFlowId)}&email=${encodeURIComponent(email.trim())}&csrf_token=${encodeURIComponent(nextCsrfToken)}`;
        } catch (err) {
            console.error('Send code error:', err);
            setFormError(err.message || 'Failed to send recovery code.');
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="forgot-main-container">
            <div className="forgot-left-panel">
                <div className="forgot-brand-container">
                    <img
                        src="/images/taksa_black.png"
                        alt="Taksa Logo"
                        className="forgot-brand-logo"
                    />
                    <h3 className="forgot-brand-title">Factory Operating System</h3>
                </div>
            </div>

            <div className="forgot-right-panel">
                <div className="forgot-form-wrapper">
                    <h2 className="forgot-header-title">Forgot Password</h2>
                    <p className="forgot-header-subtitle">
                        Enter your email address and we&apos;ll send you a 6-digit code to reset your password.
                    </p>

                    {formError && <div className="forgot-form-error">{formError}</div>}

                    {!isInitializing && (
                        <form className="forgot-auth-form" onSubmit={handleSendCode}>
                            <div className="forgot-input-group">
                                <label htmlFor="recovery-email">Email *</label>
                                <input
                                    type="email"
                                    id="recovery-email"
                                    placeholder="user@yourcompany.com"
                                    value={email}
                                    onChange={(e) => {
                                        setEmail(e.target.value);
                                        if (emailError) setEmailError('');
                                        if (formError) setFormError('');
                                    }}
                                    className={emailError ? 'forgot-input-error-border' : ''}
                                    disabled={isLoading}
                                />
                                {emailError && (
                                    <span className="forgot-error-message">{emailError}</span>
                                )}
                            </div>

                            <button
                                type="submit"
                                className="forgot-submit-btn"
                                disabled={isLoading}
                            >
                                {isLoading ? 'Sending Code...' : 'Send Recovery Code'}
                            </button>
                        </form>
                    )}

                    <div
                        className="forgot-footer-section"
                        style={{ marginTop: '20px', textAlign: 'center' }}
                    >
                            <Link href="/">← Back to Login</Link>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default ForgotPassword;