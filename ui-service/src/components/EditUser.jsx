'use client';

import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { ChevronLeft, Eye, EyeOff, CheckCircle2, AlertCircle, Loader2 } from 'lucide-react';
import './EditUser.css';

const sanitizeRoleValue = (role) => (role || '').trim();

const isMasterRole = (role) => {
    const normalizedRole = sanitizeRoleValue(role).toLowerCase();
    return normalizedRole === 'master';
};

const EditUserContent = () => {
    const searchParams = useSearchParams();
    const router = useRouter();
    const targetUserId = searchParams.get('id');
    const urlFirstName = searchParams.get('firstName') || '';
    const urlLastName = searchParams.get('lastName') || '';
    const urlEmail = searchParams.get('email') || '';


    const rawUrlRole = searchParams.get('role') || 'user';
    const urlRole = sanitizeRoleValue(rawUrlRole) || 'user';

    const [activeTab, setActiveTab] = useState('details');
    const [successMessage, setSuccessMessage] = useState('');
    const [apiError, setApiError] = useState('');
    const [isLoading, setIsLoading] = useState(false);

    const [loggedInUserRole, setLoggedInUserRole] = useState('');
    const [loggedInUserId, setLoggedInUserId] = useState(null);
    const [loggedInUserEmail, setLoggedInUserEmail] = useState('');

    const [userDetails, setUserDetails] = useState({
        firstName: urlFirstName,
        lastName: urlLastName,
        email: urlEmail,
        role: urlRole
    });

    // Removed old password verification logic

    const [newPasswords, setNewPasswords] = useState({ new: '', confirm: '' });
    const [passErrors, setPassErrors] = useState({});
    const [showOldPass, setShowOldPass] = useState(false);
    const [showNewPass, setShowNewPass] = useState(false);
    const [showConfirmPass, setShowConfirmPass] = useState(false);

    const [showRedirectModal, setShowRedirectModal] = useState(false);

    const parseJsonResponse = async (response, fallbackMessage) => {
        const raw = await response.text();
        let data = {};

        try {
            data = raw ? JSON.parse(raw) : {};
        } catch {
            if (!response.ok) {
                throw new Error(raw || fallbackMessage);
            }
            data = {};
        }

        if (!response.ok || data?.code) {
            throw new Error(
                data?.ui?.messages?.[0]?.text ||
                data?.error?.message ||
                data?.message ||
                fallbackMessage
            );
        }

        return data;
    };

    useEffect(() => {
        try {
            const storedData = localStorage.getItem('taksa_user');
            if (storedData) {
                const parsedUser = JSON.parse(storedData);
                setLoggedInUserRole(sanitizeRoleValue(parsedUser.role));
                setLoggedInUserEmail(parsedUser.email);
                setLoggedInUserId(String(parsedUser.identityId || parsedUser.identity_id || parsedUser.id));
            }
        } catch (error) {
            console.error("Error loading user data", error);
        }
    }, []);

    useEffect(() => {
        const fetchWhoAmI = async () => {
            try {
                const response = await fetch('/sessions/whoami', {
                    method: 'GET',
                    headers: {
                        Accept: 'application/json'
                    },
                    credentials: 'include'
                });

                if (!response.ok) {
                    throw new Error('Failed to fetch user session');
                }

                const data = await response.json();
                const identity = data?.identity || {};
                const traits = identity?.traits || {};
                const currentRole = sanitizeRoleValue(traits?.role);

                setLoggedInUserEmail(traits?.email || '');
                setLoggedInUserId((prev) => prev || String(identity?.id || ''));
                setLoggedInUserRole(currentRole);

                localStorage.setItem(
                    'taksa_user',
                    JSON.stringify({
                        identity_id: identity?.id || '',
                        email: traits?.email || '',
                        first_name: traits?.name?.first || '',
                        last_name: traits?.name?.last || '',
                        role: traits?.role || '',
                        organization_name: traits?.organization_name || '',
                        // camelCase aliases for compatibility with existing consumers
                        identityId: identity?.id || '',
                        firstName: traits?.name?.first || '',
                        lastName: traits?.name?.last || '',
                        organizationName: traits?.organization_name || ''
                    })
                );
            } catch (error) {
                console.error('Error loading user data from whoami:', error);
                setLoggedInUserEmail('');
            }
        };

        fetchWhoAmI();
    }, []);

    const handleDetailsChange = (e) => {
        const { name, value } = e.target;
        setUserDetails(prev => ({ ...prev, [name]: value }));
    };

    const handleSaveDetails = async (e) => {
        e.preventDefault();
        setApiError('');
        setSuccessMessage('');

        const sanitizedRole = sanitizeRoleValue(userDetails.role);
        const canManageRoles = isMasterRole(loggedInUserRole);

        if (String(loggedInUserId) === String(targetUserId) && canManageRoles && !isMasterRole(sanitizedRole)) {
            setApiError('You are the only Super-admin of this organisation.');
            return;
        }

        setIsLoading(true);

        try {
            const payload = {
                identity_id: targetUserId,
                first_name: userDetails.firstName,
                last_name: userDetails.lastName,
            };

            if (canManageRoles) {
                payload.role = sanitizedRole;
            }

            const response = await fetch(`/api/v1/um/update_profile`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify(payload)
            });

            await parseJsonResponse(response, 'Failed to update user details.');

            setSuccessMessage('User details updated successfully!');
            setTimeout(() => setSuccessMessage(''), 3000);

            router.replace(`/dashboard/users/edit?id=${targetUserId}&firstName=${encodeURIComponent(userDetails.firstName)}&lastName=${encodeURIComponent(userDetails.lastName)}&email=${encodeURIComponent(userDetails.email)}&role=${encodeURIComponent(sanitizedRole)}`);

            if (String(loggedInUserId) === String(targetUserId)) {
                setLoggedInUserRole(sanitizedRole);

                try {
                    const storedData = JSON.parse(localStorage.getItem('taksa_user') || '{}');
                    storedData.role = sanitizedRole;
                    localStorage.setItem('taksa_user', JSON.stringify(storedData));
                } catch (error) {
                    console.error('Failed updating local user role', error);
                }
            }

        } catch (error) {
            console.error("Update Profile Error:", error);
            setApiError(error.message || 'Failed to update user details.');
        } finally {
            setIsLoading(false);
        }
    };

    const initializeLoginVerificationFlow = async () => {
        const response = await fetch('/self-service/login/browser?refresh=true', {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include',
            redirect: 'follow'
        });

        const data = await parseJsonResponse(
            response,
            'Failed to initialize login verification flow'
        );

        const csrfNode = data?.ui?.nodes?.find(
            (node) => node?.attributes?.name === 'csrf_token'
        );

        return {
            flowId: data?.id || '',
            csrfToken: csrfNode?.attributes?.value || ''
        };
    };

    // Removed handleVerifyOldPassword and related logic

    const initializeSettingsFlow = async () => {
        const response = await fetch('/self-service/settings/browser', {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include',
            redirect: 'follow'
        });

        const data = await parseJsonResponse(
            response,
            'Failed to initialize settings flow'
        );

        const csrfNode = data?.ui?.nodes?.find(
            (node) => node?.attributes?.name === 'csrf_token'
        );

        return {
            flowId: data?.id || '',
            csrfToken: csrfNode?.attributes?.value || ''
        };
    };

    const handleNewPasswordChange = (e) => {
        const { name, value } = e.target;
        const updatedPasswords = { ...newPasswords, [name]: value };

        setNewPasswords(updatedPasswords);

        setPassErrors(() => {
            const newErrors = {};

            if (updatedPasswords.new && updatedPasswords.new.length < 8) {
                newErrors.new = "New password needs to be at least 8 characters long";
            }

            if (updatedPasswords.confirm && updatedPasswords.new !== updatedPasswords.confirm) {
                newErrors.confirm = "Passwords do not match";
            }

            return newErrors;
        });
    };

    const handleSaveNewPassword = async (e) => {
        e.preventDefault();
        setApiError('');
        setSuccessMessage('');
        setPassErrors({});

        // Removed old password verification check

        if (!newPasswords.new) {
            setPassErrors({ new: "New password is required" });
            return;
        }

        if (newPasswords.new.length < 8) {
            setPassErrors({ new: "New password needs to be at least 8 characters long" });
            return;
        }

        if (!newPasswords.confirm) {
            setPassErrors({ confirm: "Please confirm your new password" });
            return;
        }

        if (newPasswords.new !== newPasswords.confirm) {
            setPassErrors({ confirm: "Passwords do not match" });
            return;
        }

        setIsLoading(true);

        try {
            const { flowId, csrfToken } = await initializeSettingsFlow();

            if (!flowId) {
                throw new Error('Settings flow not initialized.');
            }

            const response = await fetch(`/self-service/settings?flow=${flowId}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    method: 'password',
                    password: newPasswords.new,
                    csrf_token: csrfToken
                })
            });

            await parseJsonResponse(response, 'Failed to update password.');

            setShowRedirectModal(true);

            setTimeout(async () => {
                try {
                    localStorage.clear();

                    const logoutRes = await fetch('/self-service/logout/browser', {
                        method: 'GET',
                        headers: {
                            Accept: 'application/json'
                        },
                        credentials: 'include'
                    });

                    const logoutData = await logoutRes.json().catch(() => ({}));

                    if (!logoutRes.ok) {
                        throw new Error(
                            logoutData?.error?.message ||
                            logoutData?.message ||
                            'Failed to initialize logout flow'
                        );
                    }

                    if (logoutData?.logout_url) {
                        const kratosLogoutUrl = new URL(logoutData.logout_url);
                        const finalLogoutUrl = `${window.location.origin}${kratosLogoutUrl.pathname}${kratosLogoutUrl.search}`;
                        window.location.href = finalLogoutUrl;
                        return;
                    }

                    window.location.href = '/';
                } catch (logoutError) {
                    console.error('Logout after password change failed:', logoutError);
                    window.location.href = '/';
                }
            }, 2500);

        } catch (error) {
            console.error("Change Password Error:", error);
            setApiError(error.message || 'Failed to update password.');
            setIsLoading(false);
        }
    };

    const currentRoleValue = sanitizeRoleValue(userDetails.role);
    const canManageRoles = isMasterRole(loggedInUserRole);

    const isDetailsModified =
        userDetails.firstName !== urlFirstName ||
        userDetails.lastName !== urlLastName ||
        currentRoleValue !== urlRole;

    return (
        <div className="edit-user-container">
            <div className="eu-header-wrapper">
                <button className="back-btn" onClick={() => router.push('/dashboard/users')}>
                    <ChevronLeft size={24} />
                </button>
                <div>
                    <h1 className="eu-title">Edit Account Details</h1>
                    <p className="eu-subtitle">Update user details and manage security settings.</p>
                </div>
            </div>
            {successMessage && (
                <div className="eu-success-alert">
                    <CheckCircle2 size={18} /> {successMessage}
                </div>
            )}
            {apiError && (
                <div className="eu-success-alert eu-top-right-alert" style={{ backgroundColor: '#fef2f2', color: '#dc2626', borderColor: '#fecaca' }}>
                    <AlertCircle size={18} /> {apiError}
                </div>
            )}

            <div className="eu-tabs-container">
                <button
                    className={`eu-tab ${activeTab === 'details' ? 'active' : ''}`}
                    onClick={() => { setActiveTab('details'); setApiError(''); }}
                >
                    Edit Details
                </button>
                <button
                    className={`eu-tab ${activeTab === 'password' ? 'active' : ''}`}
                    onClick={() => { setActiveTab('password'); setApiError(''); }}
                >
                    Edit Password
                </button>
            </div>
            {activeTab === 'details' && (
                <div className="eu-card">
                    <h3 className="eu-card-title">Account Details</h3>
                    <p className="eu-card-desc">View and edit the account information.</p>

                    <form onSubmit={handleSaveDetails} className="eu-form">
                        <div className="eu-input-row">
                            <label>First Name</label>
                            <input
                                type="text"
                                name="firstName"
                                value={userDetails.firstName}
                                onChange={handleDetailsChange}
                            />
                        </div>
                        <div className="eu-input-row">
                            <label>Last Name</label>
                            <input
                                type="text"
                                name="lastName"
                                value={userDetails.lastName}
                                onChange={handleDetailsChange}
                            />
                        </div>
                        <div className="eu-input-row">
                            <label>E-Mail</label>
                            <input
                                type="email"
                                name="email"
                                value={userDetails.email}
                                disabled={true}
                            />
                        </div>

                        {canManageRoles && (
                            <div className="eu-input-row">
                                <label>Role</label>
                                <input
                                    type="text"
                                    name="role"
                                    value={userDetails.role}
                                    onChange={handleDetailsChange}
                                    placeholder="Enter role"
                                />
                            </div>
                        )}

                        <div className="eu-actions">
                            <button
                                type="submit"
                                className="btn-save"
                                disabled={isLoading || !isDetailsModified}
                            >
                                {isLoading ? 'Saving...' : 'Save Details'}
                            </button>
                        </div>
                    </form>
                </div>
            )}
            {activeTab === 'password' && (
                <div className="eu-password-wrapper">
                  {String(loggedInUserId) !== String(targetUserId) ? (
                    <div className="eu-card" style={{ color: '#dc2626', background: '#fef2f2', border: '1px solid #fecaca' }}>
                      <h3 className="eu-card-title">Password Change Not Allowed</h3>
                      <p className="eu-card-desc">
                        Only the owner of this account can change their password.
                      </p>
                    </div>
                  ) : (
                    <div className="eu-card">
                        <h3 className="eu-card-title">Change Password</h3>
                        <p className="eu-card-desc">Choose a strong, unique password.</p>
                        <form onSubmit={handleSaveNewPassword} className="eu-form">
                            <div className="eu-input-row">
                                <label>New Password</label>
                                <div className="pass-input-wrapper">
                                    <input
                                        type={showNewPass ? "text" : "password"}
                                        name="new"
                                        value={newPasswords.new}
                                        onChange={handleNewPasswordChange}
                                        disabled={isLoading}
                                        className={passErrors.new ? 'input-error' : ''}
                                    />
                                    <button type="button" className="eye-btn" onClick={() => setShowNewPass(!showNewPass)} disabled={isLoading}>
                                        {showNewPass ? <Eye size={18} /> : <EyeOff size={18} />}
                                    </button>
                                </div>
                                {passErrors.new && <span className="error-text">{passErrors.new}</span>}
                            </div>

                            <div className="eu-input-row">
                                <label>Confirm New Password</label>
                                <div className="pass-input-wrapper">
                                    <input
                                        type={showConfirmPass ? "text" : "password"}
                                        name="confirm"
                                        value={newPasswords.confirm}
                                        onChange={handleNewPasswordChange}
                                        disabled={isLoading}
                                        className={passErrors.confirm ? 'input-error' : ''}
                                    />
                                    <button type="button" className="eye-btn" onClick={() => setShowConfirmPass(!showConfirmPass)} disabled={isLoading}>
                                        {showConfirmPass ? <Eye size={18} /> : <EyeOff size={18} />}
                                    </button>
                                </div>
                                {passErrors.confirm && <span className="error-text">{passErrors.confirm}</span>}
                            </div>

                            <div className="eu-actions">
                                <button
                                    type="submit"
                                    className="btn-save"
                                    disabled={!newPasswords.new || !!passErrors.confirm || !!passErrors.new || isLoading}
                                >
                                    {isLoading ? 'Updating...' : 'Update Password'}
                                </button>
                            </div>
                        </form>
                    </div>
                  )}
                </div>
            )}
            {showRedirectModal && (
                <div className="eu-modal-overlay">
                    <div className="eu-redirect-modal">
                        <CheckCircle2 size={48} color="#16a34a" style={{ marginBottom: '15px' }} />
                        <h3 style={{ margin: '0 0 10px 0', fontSize: '1.25rem', color: '#111827' }}>Password Changed Successfully!</h3>
                        <p style={{ margin: '0 0 20px 0', color: '#4b5563', fontSize: '0.95rem' }}>
                            Your password has been updated. You will now be redirected to the login page to sign in with your new credentials.
                        </p>
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '10px', color: '#6b7280', fontSize: '0.9rem' }}>
                            <Loader2 className="eu-spin" size={16} /> Redirecting...
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default function EditUser() {
    return (
        <Suspense fallback={<div style={{ padding: '40px' }}>Loading Data...</div>}>
            <EditUserContent />
        </Suspense>
    );
}