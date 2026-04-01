'use client';

import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ChevronLeft, Info, Copy, Check, AlertCircle, X } from 'lucide-react';
import './AddInstance.css';

const AddInstance = () => {
    const router = useRouter();
    const [instanceName, setInstanceName] = useState('');
    const [orgName, setOrgName] = useState('');
    const [level1, setLevel1] = useState('');
    const [level2, setLevel2] = useState('');
    const [level3, setLevel3] = useState('');
    const [level4, setLevel4] = useState('');

    const [showModal, setShowModal] = useState(false);
    const [hasCopied, setHasCopied] = useState(false);
    const [errors, setErrors] = useState({});

    const dockerCommand = `docker volume create umh-core-data && docker run -d --restart unless-stopped --name ${instanceName || 'umh-core'} -p 1883:1883 -p 8883:8883 -v umh-core-data:/data unitedmanufacturinghub/core:latest`;

    useEffect(() => {
        try {
            const storedData = localStorage.getItem('taksa_user');
            if (storedData) {
                const parsedUser = JSON.parse(storedData);
                setOrgName(parsedUser.organizationName || 'My Organization');
            }
        } catch (error) {
            console.error("Error loading user data", error);
        }
    }, []);

    const handleBack = () => { router.back(); };

    const handleAddInstanceClick = () => {
        const newErrors = {};
        if (!instanceName.trim()) newErrors.instanceName = "Required field";
        if (!orgName.trim()) newErrors.orgName = "Required field";

        if (Object.keys(newErrors).length > 0) {
            setErrors(newErrors);
            return;
        }
        setErrors({});
        setShowModal(true);
    };

    const handleCloseModal = () => { setShowModal(false); };

    const handleCopyCommand = async () => {
        try {
            if (navigator?.clipboard?.writeText) {
                await navigator.clipboard.writeText(dockerCommand);
                setHasCopied(true);
            } else {
                const textArea = document.createElement("textarea");
                textArea.value = dockerCommand;
                textArea.style.position = "fixed";
                textArea.style.left = "-999999px";
                textArea.style.top = "-999999px";
                document.body.appendChild(textArea);
                textArea.focus();
                textArea.select();
                try {
                    document.execCommand('copy');
                    setHasCopied(true);
                } catch (err) {
                    console.error('Fallback: Oops, unable to copy', err);
                }
                document.body.removeChild(textArea);
            }
        } catch (err) {
            console.error('Failed to copy text: ', err);
        }
    };

    const handleContinue = () => {
        if (hasCopied) {
            const existingInstances = JSON.parse(localStorage.getItem('taksa_demo_instances') || '[]');

            const newInstance = {
                id: Date.now().toString(),
                name: instanceName,
                level0: orgName,
                level1: level1,
                level2: level2,
                level3: level3,
                level4: level4,
                type: 'Core',
                version: '0.44.6',
                flows: 0,
                topics: 0,
                latency: 0,
                throughput: '0.00'
            };

            existingInstances.push(newInstance);
            localStorage.setItem('taksa_demo_instances', JSON.stringify(existingInstances));
            router.push('/dashboard/Edge-devices');
        }
    };

    return (
        <div className="add-instance-container">
            <div className="add-instance-header">
                <div className="header-left">
                    <button className="back-btn" onClick={handleBack}><ChevronLeft size={24} /></button>
                    <div>
                        <h1 className="page-title">Edge Device Setup</h1>
                        <p className="page-subtitle">Add a new edge device to your infrastructure.</p>
                    </div>
                </div>
                <button className="btn-black-header" onClick={handleAddInstanceClick}>Add Edge Device</button>
            </div>

            <div className="add-instance-content">
                <div className="setup-card">
                    <h3 className="card-title">General</h3>

                    <div className="form-row" style={{ alignItems: 'flex-start' }}>
                        <div className="label-col" style={{ marginTop: '12px' }}>Name</div>
                        <div style={{ flex: 1 }}>
                            <input
                                type="text"
                                className={`form-input ${errors.instanceName ? 'input-error' : ''}`}
                                placeholder="Give it a cool name"
                                value={instanceName}
                                onChange={(e) => {
                                    setInstanceName(e.target.value);
                                    if (errors.instanceName) setErrors({ ...errors, instanceName: null });
                                }}
                            />
                            {errors.instanceName && <div className="error-text">{errors.instanceName}</div>}
                        </div>
                    </div>

                    <div className="form-row">
                        <div className="label-col">Type</div>
                        <input type="text" className="form-input input-locked" value="Core" readOnly />
                    </div>
                    <div className="form-row">
                        <div className="label-col">
                            <Info size={16} className="info-icon" /><span>Release Channel</span>
                        </div>
                        <input type="text" className="form-input input-locked" value="Stable" readOnly />
                    </div>
                </div>

                <div className="setup-card">
                    <h3 className="card-title">Location</h3>
                    <p className="card-desc">
                        Define where this Edge Device sits in your organization. Used for data organization and topic routing.
                    </p>

                    <div className="location-row" style={{ alignItems: 'flex-start' }}>
                        <div className="location-label-col" style={{ marginTop: '12px' }}>
                            Level 0 <span className="required">*</span>
                        </div>
                        <div style={{ flex: 1 }}>
                            <input
                                type="text"
                                className={`form-input ${errors.orgName ? 'input-error' : ''}`}
                                value={orgName}
                                onChange={(e) => {
                                    setOrgName(e.target.value);
                                    if (errors.orgName) setErrors({ ...errors, orgName: null });
                                }}
                            />
                            {errors.orgName && <div className="error-text">{errors.orgName}</div>}
                        </div>
                    </div>

                    <div className="location-row">
                        <div className="location-label-col">Level 1</div>
                        <input type="text" className="form-input" placeholder="Your level 1 name" value={level1} onChange={(e) => setLevel1(e.target.value)} />
                    </div>
                    <div className="location-row">
                        <div className="location-label-col">Level 2</div>
                        <input type="text" className="form-input" placeholder="Your level 2 name" value={level2} onChange={(e) => setLevel2(e.target.value)} />
                    </div>
                    <div className="location-row">
                        <div className="location-label-col">Level 3</div>
                        <input type="text" className="form-input" placeholder="Your level 3 name" value={level3} onChange={(e) => setLevel3(e.target.value)} />
                    </div>
                    <div className="location-row">
                        <div className="location-label-col">Level 4</div>
                        <input type="text" className="form-input" placeholder="Your level 4 name" value={level4} onChange={(e) => setLevel4(e.target.value)} />
                    </div>
                </div>
            </div>

            {showModal && (
                <div className="modal-overlay">
                    <div className="modal-content">
                        <div className="modal-header-row">
                            <h2 className="modal-title">Register successful</h2>
                            <button className="modal-close-btn" onClick={handleCloseModal}><X size={20} /></button>
                        </div>
                        <p className="modal-success-text">Your Edge Device has been created.</p>
                        <div className="note-box">
                            <AlertCircle size={20} className="note-icon" />
                            <div className="note-text">
                                <span className="note-title">Note</span>
                                The installation command applies only to your current session and will be shown just once. Data is stored in a Docker volume named 'umh-core-data'. To use a different volume name, update it in the command or compose file. On Linux, you may need to prefix with 'sudo' or ensure your user is in the 'docker' group.
                            </div>
                        </div>
                        <div className="tab-track"><button className="tab-btn-pill">Docker Run</button></div>
                        <div className="code-box-container">
                            <div className="code-text-scroll">{dockerCommand}</div>
                            <button className={`copy-btn ${hasCopied ? 'copied' : ''}`} onClick={handleCopyCommand} title="Copy to clipboard">
                                {hasCopied ? <Check size={20} /> : <Copy size={20} />}
                            </button>
                        </div>
                        <div className="modal-actions">
                            <button className={`btn-continue ${hasCopied ? 'active' : 'disabled'}`} onClick={handleContinue} disabled={!hasCopied}>Continue</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default AddInstance;