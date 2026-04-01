'use client';

import React, { useEffect, useState, useRef } from 'react';
import Link from 'next/link';
import { MoreVertical } from 'lucide-react';
import './Instances.css';

const Instances = () => {
    const [instances, setInstances] = useState([]);
    const [openDropdownId, setOpenDropdownId] = useState(null);
    const dropdownRef = useRef(null);

    useEffect(() => {
        const stored = JSON.parse(localStorage.getItem('taksa_demo_instances') || '[]');
        setInstances(stored);

        const handleClickOutside = (event) => {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
                setOpenDropdownId(null);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    const toggleDropdown = (id) => {
        setOpenDropdownId(openDropdownId === id ? null : id);
    };

    const handleDelete = (id) => {
        const updatedList = instances.filter(inst => inst.id !== id);
        setInstances(updatedList);
        localStorage.setItem('taksa_demo_instances', JSON.stringify(updatedList));
        setOpenDropdownId(null);
    };

    return (
        <div className="instances-container">
            <div className="instances-header-container">
                <div>
                    <h1 className="instances-title">Edge Devices</h1>
                    <p className="instances-subtitle">
                        Set up, manage, and oversee all your edge devices. Adding a new edge device is the first step in establishing your Unified Namespace.
                    </p>
                </div>
                {instances.length > 0 && (
                    <Link href="/dashboard/Edge-devices/add" style={{ textDecoration: 'none' }}>
                        <button className="btn-black header-add-btn">Add Edge Device</button>
                    </Link>
                )}
            </div>

            {instances.length === 0 ? (
                <div className="instances-empty-state">
                    <h3 className="empty-state-bold">No Edge Devices Available</h3>
                    <p className="empty-state-sub">Welcome! Let's get started by setting up your first edge device.</p>
                    <Link href="/dashboard/Edge-devices/add" style={{ textDecoration: 'none' }}>
                        <button className="btn-black">Add Your First Edge Device</button>
                    </Link>
                </div>
            ) : (
                <div className="instances-table-wrapper">
                    <table className="instances-table">
                        <thead>
                            <tr>
                                <th className="serial-col">Serial</th>
                                <th>Edge Device Name</th>
                                <th>Type</th>
                                <th>Version</th>
                                <th>Data Flows</th>
                                <th>Topics</th>
                                <th>Latency (ms)</th>
                                <th>Read Throughput (msg/sec)</th>
                                <th className="action-col">Action</th>
                            </tr>
                        </thead>
                        <tbody>
                            {instances.map((instance, index) => (
                                <tr key={instance.id}>
                                    <td className="serial-col">{index + 1}</td>
                                    <td><span className="device-name-text">{instance.name}</span></td>

                                    {/* === FIXED: Pulling Type and Version from local storage === */}
                                    <td>{instance.type || 'n/a'}</td>
                                    <td>{instance.version || 'n/a'}</td>

                                    <td><span className="underlined-number">{instance.flows || 0}</span></td>
                                    <td><span className="underlined-number">{instance.topics || 0}</span></td>
                                    <td>{instance.latency || 0}</td>
                                    <td>{instance.throughput || '0.00'}</td>
                                    <td className="action-col">
                                        <div className="action-cell">
                                            <button className="action-btn" onClick={() => toggleDropdown(instance.id)}>
                                                <MoreVertical size={20} />
                                            </button>

                                            {openDropdownId === instance.id && (
                                                <div className="action-dropdown" ref={dropdownRef}>
                                                    <div className="dropdown-item">Device details</div>
                                                    <div className="dropdown-item text-danger" onClick={() => handleDelete(instance.id)}>
                                                        Delete
                                                    </div>
                                                </div>
                                            )}
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
};

export default Instances;