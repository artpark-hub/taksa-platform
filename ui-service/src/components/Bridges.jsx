'use client';

import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import './Bridges.css';

const Bridges = () => {
    const router = useRouter();
    const [bridges, setBridges] = useState([]);
    const [isLoading, setIsLoading] = useState(true);

    const fetchBridges = async () => {
        setIsLoading(true);

        try {
            // API call will be added here later
            setBridges([]);
        } catch (error) {
            console.error('Error fetching bridges:', error);
            setBridges([]);
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        fetchBridges();
    }, []);

    return (
        <div className="bridges-container">
            <div className="bridges-header-container">
                <div>
                    <h1 className="bridges-title">Bridge</h1>
                    <p className="bridges-subtitle">
                        Manage and customize your UMH Core bridge data flows powered by the open-source benthos-umh.
                    </p>
                </div>
                {bridges.length > 0 && (
                    <button className="bridges-btn-black bridges-header-add-btn" onClick={() => router.push('/bridges/add')}>
                        Add Bridge
                    </button>
                )}
            </div>

            {isLoading ? (
                <div className="bridges-empty-state">
                    <p className="bridges-empty-loading">Loading bridges...</p>
                </div>
            ) : bridges.length === 0 ? (
                <div className="bridges-empty-state">
                    <h3 className="bridges-empty-bold">No Bridge Data Flows Found</h3>
                    <p className="bridges-empty-sub">Get started by creating your first bridge data flow.</p>
                    <button className="bridges-btn-black" onClick={() => router.push('/bridges/add')}>
                        Add Your First Bridge
                    </button>
                </div>
            ) : (
                <div className="bridges-table-wrapper">
                    <div className="bridges-grid">
                        {bridges.map((bridge) => (
                            <div key={bridge.id} className="bridge-card">
                                <h3>{bridge.name}</h3>
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    );
};

export default Bridges;
