'use client';

import React, { useState } from 'react';
import Sidebar from './Sidebar';
import Header from './Header';
import './Dashboard.css';

export default function DashboardLayoutClient({ children }) {
    const [isCollapsed, setIsCollapsed] = useState(false);

    const toggleSidebar = () => {
        setIsCollapsed(!isCollapsed);
    };

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