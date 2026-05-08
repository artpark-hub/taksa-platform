'use client';

import React, { useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Eye, EyeOff } from 'lucide-react'; 
import LegalDocumentModal from './LegalDocumentModal';

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
    const [agreementChecked, setAgreementChecked] = useState(false);
    const [agreementError, setAgreementError] = useState('');
    const [activeLegalDocument, setActiveLegalDocument] = useState(null);

    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

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
            tenant_id: traits.tenant_id || '',
            identityId: identity.id,
            firstName: traits.name?.first || '',
            lastName: traits.name?.last || '',
            organizationName: traits.organization_name || '',
            tenantId: traits.tenant_id || ''
        }));
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

        if (!finalJwt) {
            throw new Error('No JWT token in response');
        }

        localStorage.setItem('taksa_jwt', finalJwt);
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
            orgIdData?.tenant_id ||
            orgIdData?.tenantId ||
            orgIdData?.data?.tenant_id ||
            orgIdData?.data?.tenantId ||
            orgIdData?.organization_id ||
            orgIdData?.organizationId ||
            orgIdData?.data?.organization_id ||
            orgIdData?.data?.organizationId;

        if (!organizationId) throw new Error('Organization id not found in response');

        return organizationId;
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
                    tenant_id: organizationId
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
        setAgreementError('');

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
                            disabled={isLoading || !isFormValid || !agreementChecked}
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
        </div>
    );
};

export default Register;