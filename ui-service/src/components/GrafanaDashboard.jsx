'use client';

import React from 'react';
import './GrafanaDashboard.css';

const GRAFANA_URL = '/grafana';

const GrafanaDashboard = ({ deviceId = null }) => {

    const buildSrc = () => {
        const params = new URLSearchParams({
            theme: 'light',
            orgId: '1',
            refresh: '10s',
            ...(deviceId ? { 'var-deviceId': deviceId } : {}),
        });

        return `${GRAFANA_URL}?${params.toString()}`;
    };

    return (
        <div className="grafana-wrapper">
            <div className="grafana-frame-container">
                <iframe
                    key={deviceId || 'default'}
                    src={buildSrc()}
                    title="Grafana Dashboard"
                    className="grafana-iframe"
                    frameBorder="0"
                    allowFullScreen
                />
            </div>
        </div>
    );
};

export default GrafanaDashboard;