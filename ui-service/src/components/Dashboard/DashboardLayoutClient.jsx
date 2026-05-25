'use client';

import React, { useEffect, useRef, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import Sidebar from './Sidebar';
import Header from './Header';
import './Dashboard.css';

export default function DashboardLayoutClient({ children }) {
    const [isCollapsed, setIsCollapsed] = useState(false);
    const [isAuthorizing, setIsAuthorizing] = useState(true);
    const hasValidatedOnceRef = useRef(false);
    const pathname = usePathname();
    const router = useRouter();

    useEffect(() => {
        let cancelled = false;

        const clearStoredAuth = () => {
            localStorage.removeItem('taksa_session_token');
            localStorage.removeItem('taksa_jwt');
            localStorage.removeItem('taksa_user');
        };

        const storeUserFromIdentity = (identity) => {
            if (!identity || !identity.traits) {
                return;
            }

            const traits = identity.traits;

            localStorage.setItem(
                'taksa_user',
                JSON.stringify({
                    identity_id: identity.id || '',
                    email: traits.email || '',
                    first_name: traits.name?.first || '',
                    last_name: traits.name?.last || '',
                    role: traits.role || '',
                    organization_name: traits.organization_name || '',
                    tenant_id: traits.tenant_id || '',
                    identityId: identity.id || '',
                    firstName: traits.name?.first || '',
                    lastName: traits.name?.last || '',
                    organizationName: traits.organization_name || '',
                    tenantId: traits.tenant_id || ''
                })
            );
        };

        const redirectToLogin = () => {
            clearStoredAuth();
            if (!cancelled) {
                router.replace('/');
            }
        };

        const validateSession = async () => {
            if (!hasValidatedOnceRef.current) {
                setIsAuthorizing(true);
            }

            try {
                const response = await fetch('/sessions/whoami', {
                    method: 'GET',
                    headers: {
                        Accept: 'application/json'
                    },
                    credentials: 'include'
                });

                const data = await response.json().catch(() => ({}));
                const identity = data?.identity;
                const isAuthFailure = response.status === 401 || response.status === 403;

                if (isAuthFailure || (response.ok && !identity)) {
                    redirectToLogin();
                    return;
                }

                if (!response.ok) {
                    if (!cancelled) {
                        setIsAuthorizing(false);
                    }
                    return;
                }

                storeUserFromIdentity(identity);

                if (!cancelled) {
                    setIsAuthorizing(false);
                }
            } catch (error) {
                if (!cancelled) {
                    setIsAuthorizing(false);
                }
            } finally {
                hasValidatedOnceRef.current = true;
            }
        };

        validateSession();

        return () => {
            cancelled = true;
        };
    }, [pathname, router]);

    const toggleSidebar = () => {
        setIsCollapsed(!isCollapsed);
    };

    if (isAuthorizing) {
        return (
            <div className="dashboard-auth-overlay">
                <div className="dashboard-auth-modal" role="status" aria-live="polite" aria-busy="true">
                    <div className="dashboard-auth-loader" role="img" aria-label="Validating user"></div>
                    <h3>Validating user, kindly wait.</h3>
                </div>
            </div>
        );
    }

    return (
        <div className="dashboard-container">
            <Sidebar isCollapsed={isCollapsed} toggleSidebar={toggleSidebar} />

            <div className={`dashboard-main-wrapper ${isCollapsed ? 'wrapper-collapsed' : 'wrapper-expanded'}`}>
                <Header isCollapsed={isCollapsed} />
                <main className="dashboard-main-content">
                    {children}
                </main>
            </div>
        </div>
    );
}