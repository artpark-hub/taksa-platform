import React, { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import { MoreVertical, Plus, Search, Loader2, CheckCircle, X, CornerDownRight } from 'lucide-react';
import { useInstanceContext } from './InstanceContext';
import './DataFlow.css';

const DataFlow = () => {
    const navigate = useNavigate();

    // --- CONSUME GLOBAL STATE ---
    const {
        instances,
        updateInstanceStatus,
        showDiscovery,      // From Context
        setShowDiscovery    // From Context
    } = useInstanceContext();

    // --- LOCAL UI STATE (Only for things that should reset on navigation) ---
    const [activeDropdown, setActiveDropdown] = useState(null);
    const [activeTableType, setActiveTableType] = useState(null);
    const [dropdownPos, setDropdownPos] = useState({ top: 0, left: 0 });
    const [firstName, setFirstName] = useState('');
    const [isConnecting, setIsConnecting] = useState(false);
    const [showToast, setShowToast] = useState(false);

    // --- INITIALIZATION ---
    useEffect(() => {
        const storedUser = localStorage.getItem('taksa_user');
        if (storedUser) {
            try {
                const user = JSON.parse(storedUser);
                setFirstName(user.firstName || user.first_name || '');
            } catch (error) {
                console.error("Failed to parse user data:", error);
            }
        }
    }, []);

    // --- CLICK OUTSIDE HANDLER ---
    useEffect(() => {
        const handleScroll = () => { if (activeDropdown) setActiveDropdown(null); };
        const handleClickOutside = (e) => {
            if (activeDropdown && !e.target.closest('.action-dropdown') && !e.target.closest('.action-btn') && !e.target.closest('.action-dots')) {
                setActiveDropdown(null);
                setActiveTableType(null);
            }
        };
        window.addEventListener('scroll', handleScroll, true);
        document.addEventListener('mousedown', handleClickOutside);
        return () => {
            window.removeEventListener('scroll', handleScroll, true);
            document.removeEventListener('mousedown', handleClickOutside);
        };
    }, [activeDropdown]);

    // --- ACTION BUTTON HANDLER ---
    const handleActionClick = (e, id, type) => {
        e.stopPropagation();
        if (activeDropdown === id && activeTableType === type) {
            setActiveDropdown(null);
            setActiveTableType(null);
        } else {
            const rect = e.currentTarget.getBoundingClientRect();
            setDropdownPos({
                top: rect.bottom + 5,
                left: rect.right - 145
            });
            setActiveDropdown(id);
            setActiveTableType(type);
        }
    };

    // --- DISCOVER HANDLER (Updates Global Context) ---
    const handleDiscover = () => {
        if (showDiscovery) return;
        setShowDiscovery(true); // Persists until reload
        setActiveDropdown(null);
    };

    // --- INSTANCE LOGIC ---
    const activeInstance = instances.find(inst => inst.id === activeDropdown);
    const isInstanceConnected = activeInstance?.status === 'active';

    const handleNavigation = (e, path) => {
        e.stopPropagation();
        if (isInstanceConnected) navigate(path);
    };

    const handleEstablishConnection = (e, id) => {
        e.stopPropagation();
        if (isInstanceConnected) return;

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

    const servers = [
        { id: 999, serial: '1', name: 'ArtPark Device-1', status: 'Active' }
    ];

    return (
        <div className="data-flow-page">

            {/* --- LOADING & TOAST --- */}
            {isConnecting && (
                <div className="loading-overlay">
                    <div className="loading-box">
                        <Loader2 className="spinner" size={32} />
                        <p>Establishing connection...</p>
                    </div>
                </div>
            )}

            {showToast && (
                <div className="success-toast">
                    <CheckCircle size={20} />
                    <span>Connected successfully</span>
                    <button onClick={() => setShowToast(false)} className="toast-close">
                        <X size={16} />
                    </button>
                </div>
            )}

            <div className="page-header">
                <div className="welcome-text">
                    <h1>
                        Welcome
                        {firstName && (
                            <span style={{
                                fontSize: 'medium', fontWeight: '300', color: '#000', marginLeft: '8px', WebkitTextStroke: '0.3px #000'
                            }}>
                                , {firstName}
                            </span>
                        )}
                    </h1>
                    <p>Manage your Edge Devices and connections.</p>
                </div>
            </div>

            <div className="data-flow-controls">
                <div className="search-bar">
                    <Search size={18} color="#666" />
                    <input type="text" placeholder="Filter by Device name" />
                </div>
                <button className="black-btn">
                    <Plus size={18} style={{ marginRight: '8px' }} />
                    Add Edge Device
                </button>
            </div>

            {/* --- MAIN TABLE --- */}
            <div className="table-container main-table-container">
                <table className="custom-table">
                    <thead>
                        <tr>
                            <th>Serial Number</th>
                            <th>Name</th>
                            <th>Status</th>
                            <th style={{ width: '80px', textAlign: 'center' }}>Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        {servers.map((server) => (
                            <React.Fragment key={server.id}>
                                {/* PARENT ROW */}
                                <tr className={showDiscovery ? 'parent-active' : ''}>
                                    <td>{server.serial}</td>
                                    <td className="device-name-link">
                                        {server.name}
                                    </td>
                                    <td>
                                        <span className={`status-badge ${server.status.toLowerCase()}`}>
                                            {server.status}
                                        </span>
                                    </td>
                                    <td style={{ textAlign: 'center' }}>
                                        <button className="action-dots" onClick={(e) => handleActionClick(e, server.id, 'main')}>
                                            <MoreVertical size={18} color="#666" />
                                        </button>
                                    </td>
                                </tr>

                                {/* DRAWER ROW (CHILD TABLE) */}
                                {showDiscovery && (
                                    <tr className="drawer-row fade-in-up">
                                        <td colSpan="4" className="drawer-cell">
                                            <div className="drawer-content">
                                                <div className="drawer-indicator">
                                                    <CornerDownRight size={20} color="#999" />
                                                </div>

                                                <div className="drawer-table-wrapper">
                                                    <table className="custom-table instance-table">
                                                        <thead>
                                                            <tr>
                                                                <th>Serial</th>
                                                                <th>Device Name</th>
                                                                <th>Type</th>
                                                                <th>Version</th>
                                                                <th style={{ textAlign: 'center' }}>Status</th>
                                                                <th style={{ textAlign: 'right' }}>Flows</th>
                                                                <th style={{ textAlign: 'right' }}>Topics</th>
                                                                <th style={{ textAlign: 'right' }}>Latency</th>
                                                                <th style={{ textAlign: 'right' }}>Throughput</th>
                                                                <th style={{ width: '60px', textAlign: 'center' }}>Action</th>
                                                            </tr>
                                                        </thead>
                                                        <tbody>
                                                            {instances.map((item) => {
                                                                const isConnected = item.status === 'active';
                                                                return (
                                                                    <tr key={item.id} className="instance-row">
                                                                        <td style={{ color: '#666' }}>{item.serial}</td>
                                                                        <td className="instance-name">{item.name}</td>
                                                                        <td>{item.type}</td>
                                                                        <td>
                                                                            {isConnected ? (
                                                                                <>
                                                                                    <span className="version-text">{item.version}</span>
                                                                                    {item.updateAvailable}
                                                                                </>
                                                                            ) : '--'}
                                                                        </td>
                                                                        <td style={{ textAlign: 'center' }}>
                                                                            <div className={`status-dot ${item.status}`}></div>
                                                                        </td>
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
                                                                                onClick={(e) => handleActionClick(e, item.id, 'instance')}
                                                                            >
                                                                                <MoreVertical size={16} />
                                                                            </button>
                                                                        </td>
                                                                    </tr>
                                                                );
                                                            })}
                                                        </tbody>
                                                    </table>
                                                </div>
                                            </div>
                                        </td>
                                    </tr>
                                )}
                            </React.Fragment>
                        ))}
                    </tbody>
                </table>
            </div>

            {/* --- DROPDOWN PORTAL --- */}
            {activeDropdown && createPortal(
                <div
                    className="action-dropdown"
                    style={{
                        top: `${dropdownPos.top}px`,
                        left: `${dropdownPos.left}px`,
                        position: 'fixed',
                        zIndex: 9999
                    }}
                    onClick={(e) => e.stopPropagation()}
                >
                    {activeTableType === 'main' && (
                        <>
                            <div
                                className={`dropdown-item ${showDiscovery ? 'disabled' : ''}`}
                                onClick={handleDiscover}
                                title={showDiscovery ? "Discovery already done" : ""}
                            >
                                Discover
                            </div>
                            <div className="dropdown-item" onClick={() => navigate('/InstanceDetails')}>Device Details</div>
                            <div className="dropdown-item delete">Delete</div>
                        </>
                    )}

                    {activeTableType === 'instance' && (
                        <>
                            <div
                                className={`dropdown-item ${isInstanceConnected ? 'disabled' : ''}`}
                                onClick={(e) => handleEstablishConnection(e, activeDropdown)}
                            >
                                Establish connections
                            </div>
                            <div
                                className={`dropdown-item ${!isInstanceConnected ? 'disabled' : ''}`}
                                onClick={(e) => handleNavigation(e, '/visualise')}
                            >
                                Visualize
                            </div>
                            <div className="dropdown-item delete">Delete</div>
                        </>
                    )}
                </div>,
                document.body
            )}
        </div>
    );
};

export default DataFlow;