'use client';

import React, { useEffect, useState, useRef } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import {
    Boxes, Settings, Menu, ChevronUp, Users, LogOut, LayoutDashboard, FolderKanban, Bot
} from 'lucide-react';

const Sidebar = ({ isCollapsed, toggleSidebar }) => {
    const pathname = usePathname();
    const router = useRouter();
    const [user, setUser] = useState({ firstName: '', lastName: '', email: '', role: '' });

    const [showUserMenu, setShowUserMenu] = useState(false);
    const [isLoggingOut, setIsLoggingOut] = useState(false);
    const menuRef = useRef(null);

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
                const traits = data?.identity?.traits || {};

                setUser({
                    firstName: traits?.name?.first || 'User',
                    lastName: traits?.name?.last || '',
                    email: traits?.email || '',
                    role: traits?.role || ''
                });
            } catch (error) {
                console.error("Error loading user data from whoami:", error);
                setUser({
                    firstName: 'User',
                    lastName: '',
                    email: '',
                    role: ''
                });
            }
        };

        fetchWhoAmI();

        const handleClickOutside = (event) => {
            if (menuRef.current && !menuRef.current.contains(event.target)) {
                setShowUserMenu(false);
            }
        };
        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, []);

    const handleLogout = async () => {
        if (isLoggingOut) return;

        try {
            setIsLoggingOut(true);
            setShowUserMenu(false);
            localStorage.removeItem('taksa_session_token');
            localStorage.removeItem('taksa_jwt');
            localStorage.removeItem('taksa_user');

            const res = await fetch('/self-service/logout/browser', {
                method: 'GET',
                credentials: 'include',
                headers: {
                    Accept: 'application/json'
                }
            });

            const data = await res.json().catch(() => ({}));

            if (data?.logout_url) {
                const kratosLogoutUrl = new URL(data.logout_url);
                const finalLogoutUrl = `${window.location.origin}${kratosLogoutUrl.pathname}${kratosLogoutUrl.search}`;

                await fetch(finalLogoutUrl, {
                    method: 'GET',
                    credentials: 'include',
                    cache: 'no-store',
                    redirect: 'follow'
                });
            }

            window.location.replace('/');
        } catch (err) {
            console.error('Logout failed:', err);
            window.location.replace('/');
        }
    };

    const handleLogoutMouseDownCapture = (e) => {
        e.preventDefault();
        e.stopPropagation();
        void handleLogout();
    };

    const suppressLogoutClick = (e) => {
        e.preventDefault();
        e.stopPropagation();
    };

    const handleUserManagementClick = (e) => {
        e.stopPropagation();
        router.push('/dashboard/users');
        setShowUserMenu(false);
    };

    const handleSettingsClick = (e) => {
        e.stopPropagation();
        router.push('/dashboard/settings');
        setShowUserMenu(false);
    };

    const firstLetter = user.firstName ? user.firstName.charAt(0).toUpperCase() : 'U';

    const navItems = [
        { name: 'Edge Devices', href: '/dashboard/Edge-devices', icon: Boxes },
        { name: 'Dashboard', href: '/dashboard/grafana', icon: LayoutDashboard },
        { name: 'Applications',href: '/dashboard/applications', icon: FolderKanban },
        { name: 'Copilot', href: '/dashboard/copilot', icon: Bot },
    ];

    const isUserSectionActive = pathname.startsWith('/dashboard/users') || pathname.startsWith('/dashboard/settings');

    return (
        <aside className={`dashboard-sidebar ${isCollapsed ? 'sidebar-collapsed' : 'sidebar-expanded'}`}>
            <div className="sidebar-header">
                <button onClick={toggleSidebar} className="sidebar-toggle-btn">
                    <Menu size={15} />
                </button>
                {!isCollapsed && (
                    <img src="/images/taksa_black.png" alt="Taksa" className="sidebar-logo" />
                )}
            </div>
            <nav className="sidebar-nav">
                {navItems.map((item) => {
                    const Icon = item.icon;
                    const isActive = pathname.startsWith(item.href);
                    return (
                        <Link
                            key={item.href}
                            href={item.href}
                            className={`sidebar-link ${isActive ? 'active' : ''}`}
                            title={isCollapsed ? item.name : ''}
                        >
                            <Icon className="sidebar-icon" size={20} />
                            <span className="link-text">{item.name}</span>
                        </Link>
                    );
                })}
            </nav>
            <div className="sidebar-footer" ref={menuRef}>
                {showUserMenu && (
                    <div className={`user-menu-popup ${isCollapsed ? 'collapsed-popup' : ''}`} onClick={(e) => e.stopPropagation()}>
                        <div className="popup-header">
                            <div className="user-avatar-small">{firstLetter}</div>
                            <div className="popup-user-info">
                                <span className="popup-name">{user.firstName} {user.lastName}</span>
                                <span className="popup-email">{user.email}</span>
                                {user.role && <span className="popup-email">{user.role}</span>}
                            </div>
                        </div>

                        <div className="popup-options">
                            <div
                                className="popup-item"
                                onClick={handleSettingsClick}
                                style={pathname.startsWith('/dashboard/settings') ? { backgroundColor: '#f0f0f0', color: '#000', fontWeight: '600' } : {}}
                            >
                                <Settings size={16} /> <span>Settings</span>
                            </div>
                            <div
                                className="popup-item"
                                onClick={handleUserManagementClick}
                                style={pathname.startsWith('/dashboard/users') ? { backgroundColor: '#f0f0f0', color: '#000', fontWeight: '600' } : {}}
                            >
                                <Users size={16} /> <span>User Management</span>
                            </div>

                            <div className="popup-divider"></div>
                            <button
                                type="button"
                                className="popup-item logout"
                                onMouseDownCapture={handleLogoutMouseDownCapture}
                                onClick={suppressLogoutClick}
                                disabled={isLoggingOut}
                                aria-busy={isLoggingOut}
                            >
                                <LogOut size={16} /> <span>{isLoggingOut ? 'Logging out...' : 'Logout'}</span>
                            </button>
                        </div>
                    </div>
                )}
                <div
                    className={`user-profile ${showUserMenu ? 'active' : ''} ${isUserSectionActive ? 'active-page' : ''}`}
                    onClick={() => {
                        if (!isLoggingOut) {
                            setShowUserMenu(!showUserMenu);
                        }
                    }}
                >
                    <div className="user-avatar">
                        {firstLetter}
                    </div>
                    {!isCollapsed && (
                        <>
                            <div className="user-info">
                                <span className="user-name">{user.firstName} {user.lastName}</span>
                                <span className="user-email" title={user.email}>{user.email}</span>
                            </div>
                            <ChevronUp size={16} color={isUserSectionActive ? "#ffffff" : "#666"} style={{ transform: showUserMenu ? 'rotate(0deg)' : 'rotate(180deg)', transition: 'transform 0.2s' }} />
                        </>
                    )}
                </div>
            </div>

        </aside>
    );
};

export default Sidebar;