import React, { useState } from 'react';
import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom';
import './DashboardLayout.css';
import taksaLogo from '../assets/images/taksa_black.png';
import Breadcrumbs from './Breadcrumbs';
import {
    Server,
    Settings,
    LogOut,
    User,
    PanelLeftClose,
    PanelLeftOpen
} from 'lucide-react';

const DashboardLayout = () => {
    const [isCollapsed, setIsCollapsed] = useState(false);
    const location = useLocation();
    const navigate = useNavigate();

    const handleLogout = () => {
        navigate('/login');
    };

    // --- LOGIC TO KEEP SIDEBAR ACTIVE ---
    // If we are on /data-flow, /instances, OR /visualize, this returns true
    const isDataFlowActive =
        location.pathname.includes('/data-flow') ||
        location.pathname.includes('/instances') ||
        location.pathname.includes('/visualise') ||
        location.pathname.includes('/InstanceDetails');

    return (
        <div className="dashboard-container">

            {/* SIDEBAR */}
            <aside className={`sidebar ${isCollapsed ? 'collapsed' : ''}`}>
                <div className="sidebar-header">
                    <div className="brand-wrapper">
                        <img src={taksaLogo} alt="Taksa" className="sidebar-logo" />
                    </div>
                    {!isCollapsed && (
                        <button className="toggle-btn" onClick={() => setIsCollapsed(true)}>
                            <PanelLeftClose size={20} />
                        </button>
                    )}
                </div>

                {isCollapsed && (
                    <div className="collapsed-toggle-wrap">
                        <button className="toggle-btn" onClick={() => setIsCollapsed(false)}>
                            <PanelLeftOpen size={20} />
                        </button>
                    </div>
                )}

                <nav className="sidebar-menu">
                    {/* Linked to Data Flow / Edge AI Devices */}
                    <Link to="/data-flow" className={`menu-item ${isDataFlowActive ? 'active' : ''}`}>
                        <Server size={20} />
                        {!isCollapsed && <span>Edge Devices</span>}
                    </Link>

                    <Link to="/settings" className={`menu-item ${location.pathname === '/settings' ? 'active' : ''}`}>
                        <Settings size={20} />
                        {!isCollapsed && <span>Settings</span>}
                    </Link>
                </nav>
            </aside>

            {/* MAIN CONTENT */}
            <main className="main-content">
                <header className="top-header">
                    <Breadcrumbs />
                    <div className="header-right">
                        <div className="user-icon">
                            <User size={20} color="#fff" />
                        </div>
                    </div>
                </header>

                <div className="page-body">
                    <Outlet />
                </div>
            </main>
        </div>
    );
};

export default DashboardLayout;