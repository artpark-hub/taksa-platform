'use client';

import React from 'react';
import './Connection.css';

const Connection = ({ bridgeConfig, setBridgeConfig }) => {
    const handleInputChange = (event) => {
        const { name, value } = event.target;

        setBridgeConfig((prev) => ({
            ...prev,
            [name]: value
        }));
    };

    return (
        <div className="bridge-connection-card">
            <div className="bridge-connection-header">
                <h2>Connection Configuration</h2>
                <p>Target system network address</p>
            </div>

            <div className="bridge-connection-form">
                <div className="bridge-connection-form-row">
                    <label>
                        IP Address
                        <span>*</span>
                    </label>

                    <input
                        type="text"
                        name="ipAddress"
                        placeholder="192.168.1.1"
                        value={bridgeConfig.ipAddress}
                        onChange={handleInputChange}
                    />
                </div>

                <div className="bridge-connection-form-row">
                    <label>
                        Port
                        <span>*</span>
                    </label>

                    <input
                        type="text"
                        name="port"
                        placeholder="502"
                        value={bridgeConfig.port}
                        onChange={handleInputChange}
                    />
                </div>
            </div>
        </div>
    );
};

export default Connection;