'use client';

import React, { useEffect, useState } from 'react';
import './Home.css';

const getDisplayName = () => {
    try {
        const storedUser = localStorage.getItem('taksa_user');

        if (!storedUser) return 'User';

        const parsedUser = JSON.parse(storedUser);

        return (
            parsedUser.firstName ||
            parsedUser.first_name ||
            parsedUser.name?.first ||
            (parsedUser.email ? parsedUser.email.split('@')[0] : '') ||
            'User'
        );
    } catch (error) {
        console.error('Error reading taksa_user from localStorage:', error);
        return 'User';
    }
};

const topCards = [
    'Total Topics',
    'Bridge Throughput',
    'Active Data Sources',
];

function Home() {
    const [displayName, setDisplayName] = useState('User');

    useEffect(() => {
        setDisplayName(getDisplayName());
    }, []);

    return (
        <div className="home-container">
            <div className="home-header-wrapper">
                <h1 className="home-page-title">Home</h1>
                <p className="home-page-subtitle">
                    Hi {displayName}, Welcome to Taksa — where factories evolve into intelligent living systems.
                </p>
            </div>

            <section className="home-top-grid">
                {topCards.map((title) => (
                    <div className="home-card" key={title}>
                        <div className="home-card-header">
                            <h2 className="home-card-title">{title}</h2>
                        </div>
                        <div className="home-card-body"></div>
                    </div>
                ))}
            </section>

            <section className="home-bottom-grid">
                <div className="home-panel">
                    <div className="home-panel-header">
                        <h2 className="home-panel-title">Current Alerts</h2>
                    </div>
                    <div className="home-panel-body"></div>
                </div>

                <div className="home-panel">
                    <div className="home-panel-header">
                        <h2 className="home-panel-title">Alert Description</h2>
                    </div>
                    <div className="home-panel-body"></div>
                </div>
            </section>
        </div>
    );
}

export default Home;