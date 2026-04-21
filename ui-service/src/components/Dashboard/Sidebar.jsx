'use client';

import React, { useEffect, useState, useRef } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import {
    Boxes, Settings, Menu, ChevronUp, Users, LogOut, House, LayoutDashboard, Workflow, FileCog, FileText, BookOpen, Compass, Bot
} from 'lucide-react';

const Sidebar = ({ isCollapsed, toggleSidebar }) => {
    const pathname = usePathname();
    const router = useRouter();
    const [user, setUser] = useState({ firstName: '', lastName: '', email: '', role: '' });

    const [showUserMenu, setShowUserMenu] = useState(false);
    const [isLoggingOut, setIsLoggingOut] = useState(false);
    const [showDashboardSubmenu, setShowDashboardSubmenu] = useState(false);
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

    useEffect(() => {
        if (pathname.startsWith('/dashboard/explore')) {
            setShowDashboardSubmenu(true);
        } else {
            setShowDashboardSubmenu(false);
        }
    }, [pathname]);

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

    const navSections = [
        {
            title: '',
            items: [
                {
                    name: 'Home',
                    href: '/dashboard/home',
                    icon: House,
                    matchPaths: ['/dashboard/home']
                }
            ]
        },
        {
            title: 'Apps',
            items: [
                {
                    name: 'Dashboard',
                    href: '/dashboard/grafana',
                    icon: LayoutDashboard,
                    matchPaths: ['/dashboard/grafana', '/dashboard/explore'],
                    children: [
                        {
                            name: 'Explore',
                            href: '/dashboard/explore',
                            icon: Compass,
                            matchPaths: ['/dashboard/explore']
                        }
                    ]
                },
                {
                    name: 'Copilot',
                    href: '/dashboard/copilot',
                    icon: Bot,
                    matchPaths: ['/dashboard/copilot']
                },
                {
                    name: 'Topic Browser',
                    href: '/dashboard/topic-browser',
                    icon: BookOpen,
                    matchPaths: ['/dashboard/topic-browser']
                }
            ]
        },
        {
            title: 'Setup',
            items: [
                {
                    name: 'DCD',
                    href: '/dashboard/Edge-devices',
                    icon: Boxes,
                    matchPaths: ['/dashboard/Edge-devices', '/dashboard/instances']
                },
                {
                    name: 'Data Flows',
                    href: '/dashboard/data-flows',
                    icon: Workflow,
                    matchPaths: ['/dashboard/data-flows']
                },
                {
                    name: 'Models',
                    href: '/dashboard/models',
                    icon: FileCog,
                    matchPaths: ['/dashboard/models']
                },
                {
                    name: 'Contracts',
                    href: '/dashboard/contracts',
                    icon: FileText,
                    matchPaths: ['/dashboard/contracts']
                }
            ]
        }
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
                {navSections.map((section, sectionIndex) => (
                    <div
                        key={`section-${sectionIndex}`}
                        className={`sidebar-section ${!section.title ? 'sidebar-section-no-title' : ''}`}
                    >
                        {section.title && (
                            <div className="sidebar-section-title">{section.title}</div>
                        )}

                        <div className="sidebar-section-links">
                            {section.items.map((item) => {
                                const Icon = item.icon;
                                const isActive = item.matchPaths.some((path) => pathname.startsWith(path));

                                if (item.children) {
                                    const isParentRouteActive = item.href ? pathname.startsWith(item.href) : false;
                                    const isChildRouteActive = item.children.some((child) =>
                                        child.matchPaths.some((path) => pathname.startsWith(path))
                                    );
                                    const parentLinkClass = isChildRouteActive
                                        ? 'dashboard-parent-active'
                                        : isParentRouteActive
                                        ? 'active'
                                        : '';
                                    const toggleClass = parentLinkClass === 'active'
                                        ? 'dashboard-toggle-active'
                                        : parentLinkClass === 'dashboard-parent-active'
                                        ? 'dashboard-toggle-muted'
                                        : '';

                                    return (
                                        <div key={item.name}>
                                            <div className={`dashboard-parent-wrap ${isCollapsed ? 'dashboard-parent-wrap-collapsed' : ''}`}>
                                                <Link
                                                    href={item.href}
                                                    className={`sidebar-link dashboard-parent-link-row ${parentLinkClass}`}
                                                    title={isCollapsed ? item.name : ''}
                                                    onClick={(e) => { e.stopPropagation(); }}
                                                >
                                                    <Icon className="sidebar-icon" size={20} />
                                                    <span className="link-text">{item.name}</span>
                                                </Link>

                                                {!isCollapsed && (
                                                    <button
                                                        type="button"
                                                        className={`dashboard-toggle-btn ${toggleClass}`}
                                                        onClick={(e) => {
                                                            e.preventDefault();
                                                            e.stopPropagation();
                                                            setShowDashboardSubmenu(!showDashboardSubmenu);
                                                        }}
                                                        aria-label="Toggle dashboard submenu"
                                                        aria-expanded={showDashboardSubmenu}
                                                    >
                                                        <ChevronUp
                                                            size={16}
                                                            color="currentColor"
                                                            style={{
                                                                transform: showDashboardSubmenu ? 'rotate(180deg)' : 'rotate(0deg)',
                                                                transition: 'transform 0.2s',
                                                                flexShrink: 0
                                                            }}
                                                        />
                                                    </button>
                                                )}
                                            </div>

                                            {!isCollapsed && showDashboardSubmenu && (
                                                <div
                                                    style={{
                                                        display: 'flex',
                                                        flexDirection: 'column',
                                                        gap: 'var(--space-xs)',
                                                        marginTop: '0.2rem'
                                                    }}
                                                >
                                                    {item.children.map((child) => {
                                                        const ChildIcon = child.icon;
                                                        const isChildActive = child.matchPaths.some((path) => pathname.startsWith(path));

                                                        return (
                                                            <Link
                                                                key={child.href}
                                                                href={child.href}
                                                                className={`sidebar-link ${isChildActive ? 'active' : ''}`}
                                                                style={{
                                                                    marginLeft: '1.75rem',
                                                                    width: 'calc(100% - 1.75rem)',
                                                                    paddingTop: '0.6rem',
                                                                    paddingBottom: '0.6rem'
                                                                }}
                                                            >
                                                                <ChildIcon className="sidebar-icon" size={18} />
                                                                <span className="link-text">{child.name}</span>
                                                            </Link>
                                                        );
                                                    })}
                                                </div>
                                            )}
                                        </div>
                                    );
                                }

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
                        </div>
                    </div>
                ))}
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
                            <ChevronUp size={16} color={isUserSectionActive ? "#ffffff" : "#666"} style={{ transform: showUserMenu ? 'rotate(180deg)' : 'rotate(0deg)', transition: 'transform 0.2s' }} />
                        </>
                    )}
                </div>
            </div>

        </aside>
    );
};

export default Sidebar;