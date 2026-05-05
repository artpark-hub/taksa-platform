'use client';

import React, { useEffect, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import Sidebar from './Sidebar';
import Header from './Header';
import './Dashboard.css';

export default function DashboardLayoutClient({ children }) {
    const [isCollapsed, setIsCollapsed] = useState(false);
    const [isAuthorizing, setIsAuthorizing] = useState(true);
    const pathname = usePathname();
    const router = useRouter();

    useEffect(() => {
        let cancelled = false;

        const clearStoredAuth = () => {
            localStorage.removeItem('taksa_session_token');
            localStorage.removeItem('taksa_jwt');
            localStorage.removeItem('taksa_user');
        };

        const validateSession = async () => {
            setIsAuthorizing(true);

            try {
                const response = await fetch('/sessions/whoami', {
                    method: 'GET',
                    headers: {
                        Accept: 'application/json'
                    },
                    credentials: 'include'
                });

                const data = await response.json().catch(() => ({}));

                if (!response.ok || !data?.identity) {
                    throw new Error('Invalid user session');
                }

                if (!cancelled) {
                    setIsAuthorizing(false);
                }
            } catch (error) {
                clearStoredAuth();

                if (!cancelled) {
                    router.replace('/');
                }
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
            <div className="dashboard-container">
                <div className="dashboard-main-wrapper wrapper-expanded">
                    <main className="dashboard-main-content">Validating session...</main>
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