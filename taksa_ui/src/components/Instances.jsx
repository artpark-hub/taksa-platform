import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, MoreVertical, Search, Loader2, CheckCircle, X } from 'lucide-react';
import { useInstanceContext } from './InstanceContext'; // Import the hook
import './Instances.css';

const Instances = () => {
    const navigate = useNavigate();

    // Use Global Context for persistence
    const { instances, updateInstanceStatus } = useInstanceContext();

    const [activeDropdown, setActiveDropdown] = useState(null);
    const [dropdownPos, setDropdownPos] = useState({ top: 0, left: 0 });

    // Local UI State
    const [isConnecting, setIsConnecting] = useState(false);
    const [showToast, setShowToast] = useState(false);

    // Close dropdown on click outside
    useEffect(() => {
        const handleClose = (event) => {
            if (!event.target.closest('.action-dropdown') && !event.target.closest('.action-btn')) {
                setActiveDropdown(null);
            }
        };
        const handleScroll = () => {
            if (activeDropdown) setActiveDropdown(null);
        };
        document.addEventListener('mousedown', handleClose);
        window.addEventListener('scroll', handleScroll, true);
        return () => {
            document.removeEventListener('mousedown', handleClose);
            window.removeEventListener('scroll', handleScroll, true);
        };
    }, [activeDropdown]);

    const handleActionClick = (e, id) => {
        e.stopPropagation();
        if (activeDropdown === id) {
            setActiveDropdown(null);
        } else {
            const rect = e.currentTarget.getBoundingClientRect();
            setDropdownPos({
                top: rect.bottom + 5,
                left: rect.right - 140
            });
            setActiveDropdown(id);
        }
    };

    // --- HELPER: GET SELECTED INSTANCE STATUS ---
    // We need to find the instance corresponding to the open dropdown to know its status
    const selectedInstance = instances.find(inst => inst.id === activeDropdown);
    const isSelectedConnected = selectedInstance?.status === 'active';

    // --- NAVIGATION HANDLER ---
    const handleNavigation = (e, path) => {
        e.stopPropagation();
        if (isSelectedConnected) {
            navigate(path);
        }
    };

    // --- ESTABLISH CONNECTION HANDLER ---
    const handleEstablishConnection = (e, id) => {
        e.stopPropagation();

        // Prevent action if already connected
        if (isSelectedConnected) return;

        setActiveDropdown(null);
        setIsConnecting(true);
        updateInstanceStatus(id, 'connecting');

        setTimeout(() => {
            setIsConnecting(false);
            updateInstanceStatus(id, 'active');
            setShowToast(true);
            setTimeout(() => setShowToast(false), 3000);
        }, 5000);
    };

    return (
        <div className="instances-page">
            {/* Loading Overlay */}
            {isConnecting && (
                <div className="loading-overlay">
                    <div className="loading-box">
                        <Loader2 className="spinner" size={32} />
                        <p>Establishing connection...</p>
                    </div>
                </div>
            )}

            {/* Success Toast */}
            {showToast && (
                <div className="success-toast">
                    <CheckCircle size={20} />
                    <span>Connected successfully</span>
                    <button onClick={() => setShowToast(false)} className="toast-close">
                        <X size={16} />
                    </button>
                </div>
            )}

            <div className="instances-header">
                <div className="header-left">
                    <h1>Factory Floor Devices</h1>
                    <p>Set up, manage, and oversee all your factory floor devices.</p>
                </div>
            </div>

            <div className="instances-controls">
                <div className="search-bar">
                    <Search size={18} color="#666" />
                    <input type="text" placeholder="Filter by device name or type" />
                </div>
                <button className="white-btn">
                    <Plus size={18} />
                    Add Factory Floor Device
                </button>
            </div>

            <div className="instances-table-container">
                <table className="instances-table">
                    <thead>
                        <tr>
                            <th>Serial Number</th>
                            <th>Device Name</th>
                            <th>Type</th>
                            <th>Version</th>
                            <th style={{ textAlign: 'center' }}>Status</th>
                            <th style={{ textAlign: 'right' }}>Data Flows</th>
                            <th style={{ textAlign: 'right' }}>Topics</th>
                            <th style={{ textAlign: 'right' }}>Latency (ms)</th>
                            <th style={{ textAlign: 'right' }}>Throughput</th>
                            <th style={{ width: '60px', textAlign: 'center' }}>Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        {instances.map((item) => {
                            // Check if this specific row is active
                            const isConnected = item.status === 'active';

                            return (
                                <tr key={item.id}>
                                    <td style={{ color: '#666' }}>{item.serial}</td>
                                    <td className="instance-name">{item.name}</td>
                                    <td>{item.type}</td>

                                    {/* --- MASKED VERSION COLUMN --- */}
                                    {/* Shows version only if connected, otherwise shows '--' */}
                                    <td>
                                        {isConnected ? (
                                            <>
                                                <span className="version-text">{item.version}</span>
                                                {item.updateAvailable && <span className="update-pill">Update</span>}
                                            </>
                                        ) : (
                                            '--'
                                        )}
                                    </td>

                                    <td style={{ textAlign: 'center' }}>
                                        <div className={`status-dot ${item.status}`}></div>
                                    </td>

                                    {/* --- MASKED DATA COLUMNS --- */}
                                    {/* If connected, show data. If not, show '--' */}
                                    <td style={{ textAlign: 'right' }} className={isConnected ? "link-text" : ""}>
                                        {isConnected ? item.dataFlows : '--'}
                                    </td>
                                    <td style={{ textAlign: 'right' }} className={isConnected ? "link-text" : ""}>
                                        {isConnected ? item.topics : '--'}
                                    </td>
                                    <td style={{ textAlign: 'right' }}>
                                        {isConnected ? item.latency : '--'}
                                    </td>
                                    <td style={{ textAlign: 'right' }}>
                                        {isConnected ? item.throughput.toFixed(2) : '--'}
                                    </td>

                                    <td style={{ textAlign: 'center' }}>
                                        <button
                                            className="action-btn"
                                            onClick={(e) => handleActionClick(e, item.id)}
                                        >
                                            <MoreVertical size={18} />
                                        </button>
                                    </td>
                                </tr>
                            );
                        })}
                    </tbody>
                </table>

                {/* DROPDOWN MENU */}
                {activeDropdown && (
                    <div
                        className="action-dropdown"
                        style={{
                            position: 'fixed',
                            top: `${dropdownPos.top}px`,
                            left: `${dropdownPos.left}px`,
                            zIndex: 9999
                        }}
                    >
                        {/* 1. Establish: Disabled if ALREADY connected */}

                        <div
                            className={`dropdown-item ${isSelectedConnected ? 'disabled' : ''}`}
                            onClick={(e) => handleEstablishConnection(e, activeDropdown)}
                            title={isSelectedConnected ? "Connection already established" : ""}
                        >
                            Establish connections
                        </div>
                        <div
                            className={`dropdown-item ${!isSelectedConnected ? 'disabled' : ''}`}
                            onClick={(e) => handleNavigation(e, '/visualise')}
                            title={!isSelectedConnected ? "Only available after establishing connections" : ""}
                        >
                            Visualize
                        </div>

                        <div className="dropdown-item delete">Delete</div>
                    </div>
                )}
            </div>
        </div>
    );
};

export default Instances;