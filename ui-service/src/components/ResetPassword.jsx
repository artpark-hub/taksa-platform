'use client';

import React, { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import './ResetPassword.css';

const ResetPassword = () => {
    const [email, setEmail] = useState('');
    const [recoveryFlowId, setRecoveryFlowId] = useState('');
    const [recoveryCsrfToken, setRecoveryCsrfToken] = useState('');

    const [settingsFlowId, setSettingsFlowId] = useState('');
    const [settingsCsrfToken, setSettingsCsrfToken] = useState('');

    const [code, setCode] = useState('');
    const [step, setStep] = useState('code'); // code | reset | success

    const [newPassword, setNewPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [showNewPassword, setShowNewPassword] = useState(false);
    const [showConfirmPassword, setShowConfirmPassword] = useState(false);

    const [isLoading, setIsLoading] = useState(false);
    const [isInitializing, setIsInitializing] = useState(true);
    const [formError, setFormError] = useState('');

    const didInitRef = useRef(false);

    const EyeOpenIcon = () => (
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
            <circle cx="12" cy="12" r="3"></circle>
        </svg>
    );

    const EyeClosedIcon = () => (
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
            <line x1="1" y1="1" x2="23" y2="23"></line>
        </svg>
    );

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

    const tryLoadSettingsFlow = async (flowId) => {
        const response = await fetch(`/self-service/settings/flows?id=${encodeURIComponent(flowId)}`, {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include',
            cache: 'no-store'
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            return false;
        }

        setSettingsFlowId(data?.id || flowId);
        setSettingsCsrfToken(extractCsrfToken(data));
        setStep('reset');
        return true;
    };

    useEffect(() => {
        if (didInitRef.current) return;
        didInitRef.current = true;

        const initialize = async () => {
            setIsInitializing(true);
            setFormError('');

            try {
                const params = new URLSearchParams(window.location.search);

                const flowFromUrl = params.get('flow') || '';
                const emailFromUrl = params.get('email') || '';
                const csrfFromUrl = params.get('csrf_token') || '';

                const storedFlow = sessionStorage.getItem('recovery_flow_id') || '';
                const storedCsrf = sessionStorage.getItem('recovery_csrf_token') || '';
                const storedEmail = sessionStorage.getItem('recovery_email') || '';

                const finalFlow = flowFromUrl || storedFlow;
                const finalEmail = emailFromUrl || storedEmail;
                const finalCsrf = csrfFromUrl || storedCsrf;

                if (!finalFlow) {
                    throw new Error('Recovery flow is missing. Please start again.');
                }

                // FIRST: if this is already a settings flow after Kratos redirect,
                // switch straight to reset-password step.
                const loadedSettings = await tryLoadSettingsFlow(finalFlow);
                if (loadedSettings) {
                    setEmail(finalEmail);
                    return;
                }

                // OTHERWISE treat it as recovery flow for OTP verification.
                if (!finalCsrf) {
                    throw new Error('Recovery CSRF token is missing. Please start again.');
                }

                setRecoveryFlowId(finalFlow);
                setRecoveryCsrfToken(finalCsrf);
                setEmail(finalEmail);
            } catch (err) {
                console.error('Reset init error:', err);
                setFormError(err.message || 'Unable to initialize reset password flow.');
            } finally {
                setIsInitializing(false);
            }
        };

        initialize();
    }, []);

    const initSettingsFlow = async () => {
        const response = await fetch('/self-service/settings/browser', {
            method: 'GET',
            headers: { Accept: 'application/json' },
            credentials: 'include',
            cache: 'no-store'
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(getErrorMessage(data, 'Failed to initialize settings flow'));
        }

        const flowId = data?.id;
        if (!flowId) {
            throw new Error('Settings flow id not found');
        }

        setSettingsFlowId(flowId);
        setSettingsCsrfToken(extractCsrfToken(data));
        setStep('reset');
        replaceUrlWithFlow(flowId);
    };

    const handleVerifyCode = async (e) => {
        e.preventDefault();
        setFormError('');

        if (!code || code.length !== 6) {
            setFormError('Please enter the 6-digit code sent to your email.');
            return;
        }

        if (!recoveryFlowId) {
            setFormError('Recovery flow is missing. Please start again.');
            return;
        }

        if (!recoveryCsrfToken) {
            setFormError('Recovery CSRF token is missing. Please start again.');
            return;
        }

        setIsLoading(true);

        try {
            const response = await fetch(
                `/self-service/recovery?flow=${encodeURIComponent(recoveryFlowId)}`,
                {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        Accept: 'application/json'
                    },
                    credentials: 'include',
                    body: JSON.stringify({
                        method: 'code',
                        code: code.trim(),
                        csrf_token: recoveryCsrfToken
                    })
                }
            );

            const data = await response.json().catch(() => ({}));

            const redirectTo =
                data?.redirect_browser_to ||
                data?.redirect_to ||
                data?.continue_with?.find?.((item) => item?.redirect_browser_to)?.redirect_browser_to ||
                data?.continue_with?.[0]?.redirect_browser_to;

            if (redirectTo) {
                window.location.href = redirectTo;
                return;
            }

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Invalid code or failed recovery'));
            }

            await initSettingsFlow();
        } catch (err) {
            console.error('Verify code error:', err);
            setFormError(err.message || 'Invalid code or failed recovery.');
        } finally {
            setIsLoading(false);
        }
    };
    
    const handleResetPassword = async (e) => {
        e.preventDefault();
        setFormError('');

        if (!newPassword) {
            setFormError('New password is required.');
            return;
        }

        if (newPassword.length < 8) {
            setFormError('Password must be at least 8 characters long.');
            return;
        }

        if (!confirmPassword) {
            setFormError('Please confirm your new password.');
            return;
        }

        if (newPassword !== confirmPassword) {
            setFormError('Passwords do not match.');
            return;
        }

        if (!settingsFlowId) {
            setFormError('Reset session is missing. Please start again.');
            return;
        }

        if (!settingsCsrfToken) {
            setFormError('Settings CSRF token is missing. Please start again.');
            return;
        }

        setIsLoading(true);

        try {
            const response = await fetch(`/self-service/settings?flow=${encodeURIComponent(settingsFlowId)}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    method: 'password',
                    password: newPassword,
                    csrf_token: settingsCsrfToken
                })
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to reset password'));
            }

            setStep('success');

            sessionStorage.removeItem('recovery_flow_id');
            sessionStorage.removeItem('recovery_csrf_token');
            sessionStorage.removeItem('recovery_email');

            setTimeout(() => {
                localStorage.removeItem('taksa_session_token');
                localStorage.removeItem('taksa_jwt');
                localStorage.removeItem('taksa_user');
                window.location.href = '/self-service/login/browser';
            }, 2500);
        } catch (err) {
            console.error('Reset password error:', err);
            setFormError(err.message || 'Failed to reset password.');
        } finally {
            setIsLoading(false);
        }
    };

    const handleRestart = () => {
        sessionStorage.removeItem('recovery_flow_id');
        sessionStorage.removeItem('recovery_csrf_token');
        sessionStorage.removeItem('recovery_email');
        window.location.href = '/recovery';
    };

    return (
        <div className="reset-main-container">
            <div className="reset-left-panel">
                <div className="reset-brand-container">
                    <img
                        src="/images/taksa_black.png"
                        alt="Taksa Logo"
                        className="reset-brand-logo auth-logo-light"
                    />
                    <img
                        src="/taksa_white_rmbg.png"
                        alt="Taksa Logo"
                        className="reset-brand-logo auth-logo-dark"
                    />
                    <h3 className="reset-brand-title">Factory Operating System</h3>
                </div>
            </div>

            <div className="reset-right-panel">
                <div className="reset-form-wrapper">
                    {step === 'success' ? (
                        <>
                            <h2 className="reset-header-title">Password Reset!</h2>
                            <div className="reset-success-box" style={{ marginTop: '20px' }}>
                                <span className="reset-success-icon">✅</span>
                                <strong>Success!</strong>
                                <p style={{ margin: '8px 0 0 0' }}>
                                    Your password has been successfully updated. Redirecting to login...
                                </p>
                            </div>
                        </>
                    ) : step === 'reset' ? (
                        <>
                            <h2 className="reset-header-title">Create New Password</h2>
                            <p className="reset-header-subtitle">
                                Recovery successful. Enter your new password below.
                            </p>

                            {formError && <div className="reset-form-error">{formError}</div>}

                            <form className="reset-auth-form" onSubmit={handleResetPassword}>
                                <div className="reset-input-group">
                                    <label htmlFor="new-password">New Password *</label>
                                    <div className="reset-password-wrapper" style={{ position: 'relative' }}>
                                        <input
                                            type={showNewPassword ? 'text' : 'password'}
                                            id="new-password"
                                            placeholder="Enter new password"
                                            value={newPassword}
                                            onChange={(e) => {
                                                setNewPassword(e.target.value);
                                                if (formError) setFormError('');
                                            }}
                                            disabled={isLoading}
                                            style={{ paddingRight: '45px', width: '100%' }}
                                        />
                                        <button
                                            type="button"
                                            className="reset-password-toggle"
                                            onClick={() => setShowNewPassword(!showNewPassword)}
                                            aria-label={showNewPassword ? "Hide password" : "Show password"}
                                            aria-pressed={showNewPassword}
                                            style={{
                                                position: 'absolute',
                                                right: '10px',
                                                top: '10px',
                                                background: 'none',
                                                border: 'none',
                                                cursor: 'pointer'
                                            }}
                                        >
                                            {showNewPassword ? <EyeOpenIcon /> : <EyeClosedIcon />}
                                        </button>
                                    </div>
                                </div>

                                <div className="reset-input-group">
                                    <label htmlFor="confirm-password">Confirm Password *</label>
                                    <div className="reset-password-wrapper" style={{ position: 'relative' }}>
                                        <input
                                            type={showConfirmPassword ? 'text' : 'password'}
                                            id="confirm-password"
                                            placeholder="Confirm new password"
                                            value={confirmPassword}
                                            onChange={(e) => {
                                                setConfirmPassword(e.target.value);
                                                if (formError) setFormError('');
                                            }}
                                            disabled={isLoading}
                                            style={{ paddingRight: '45px', width: '100%' }}
                                        />
                                        <button
                                            type="button"
                                            className="reset-password-toggle"
                                            onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                                            aria-label={showConfirmPassword ? "Hide password" : "Show password"}
                                            aria-pressed={showConfirmPassword}
                                            style={{
                                                position: 'absolute',
                                                right: '10px',
                                                top: '10px',
                                                background: 'none',
                                                border: 'none',
                                                cursor: 'pointer'
                                            }}
                                        >
                                            {showConfirmPassword ? <EyeOpenIcon /> : <EyeClosedIcon />}
                                        </button>
                                    </div>
                                </div>

                                <button type="submit" className="reset-submit-btn" disabled={isLoading}>
                                    {isLoading ? 'Resetting...' : 'Reset Password'}
                                </button>
                            </form>
                        </>
                    ) : (
                        <>
                            <h2 className="reset-header-title">Enter Recovery Code</h2>
                            <p className="reset-header-subtitle">
                                We sent a 6-digit code{email ? ` to ${email}` : ''}. Please enter it below.
                            </p>

                            {formError && <div className="reset-form-error">{formError}</div>}

                            {!isInitializing && (
                                <form className="reset-auth-form" onSubmit={handleVerifyCode}>
                                    <div className="reset-input-group">
                                        <label htmlFor="otp-code">6-Digit Code *</label>
                                        <input
                                            type="text"
                                            id="otp-code"
                                            placeholder="123456"
                                            value={code}
                                            onChange={(e) => {
                                                const value = e.target.value.replace(/\D/g, '');
                                                setCode(value);
                                                if (formError) setFormError('');
                                            }}
                                            maxLength="6"
                                            disabled={isLoading}
                                        />
                                    </div>

                                    <button type="submit" className="reset-submit-btn" disabled={isLoading}>
                                        {isLoading ? 'Verifying...' : 'Verify Code'}
                                    </button>

                                    <button
                                        type="button"
                                        onClick={handleRestart}
                                        style={{
                                            marginTop: '15px',
                                            background: 'transparent',
                                            border: 'none',
                                            color: '#0066cc',
                                            cursor: 'pointer',
                                            width: '100%'
                                        }}
                                    >
                                        Didn&apos;t get the code? Try again
                                    </button>
                                </form>
                            )}
                        </>
                    )}

                    <div
                        className="reset-footer-section"
                        style={{ marginTop: '20px', textAlign: 'center' }}
                    >
                        <Link href="/self-service/login/browser">← Back to Login</Link>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default ResetPassword;