'use client';

import React, { useEffect, useState } from 'react';
import { Bell } from 'lucide-react';

const Header = ({ isCollapsed }) => {
    const [firstName, setFirstName] = useState('');

    useEffect(() => {
        try {
            const storedData = localStorage.getItem('taksa_user');
            if (storedData) {
                const parsedUser = JSON.parse(storedData);
                setFirstName(parsedUser.firstName || 'User');
            }
        } catch (error) {
            console.error("Error loading user data", error);
        }
    }, []);

    return (
        <header className="dashboard-header">
            <div className="header-left">
                {isCollapsed && (
                    <div className="header-logo-container">
                        <img
                            src="/images/taksa_black.png"
                            alt="Taksa Logo"
                            className="header-logo"
                        />
                    </div>
                )}
            </div>
        </header>
    );
};

export default Header;