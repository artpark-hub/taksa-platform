'use client';

import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ChevronLeft, Copy, Check, AlertCircle, X } from 'lucide-react';
import './AddInstance.css';

const AddInstance = () => {
    const router = useRouter();
    const [instanceName, setInstanceName] = useState('');
    const [orgName, setOrgName] = useState('');
    const [level1, setLevel1] = useState('');
    const [level2, setLevel2] = useState('');
    const [level3, setLevel3] = useState('');
    const [level4, setLevel4] = useState('');
    const [createdBy, setCreatedBy] = useState('');

    const [showModal, setShowModal] = useState(false);
    const [hasCopied, setHasCopied] = useState(false);
    const [errors, setErrors] = useState({});
    const [formError, setFormError] = useState('');
    const [copyError, setCopyError] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [createdDeviceResponse, setCreatedDeviceResponse] = useState(null);

    const dockerCommand = createdDeviceResponse?.instructions?.docker_command || '';

    useEffect(() => {
        try {
            const storedData = localStorage.getItem('taksa_user');
            if (storedData) {
                const parsedUser = JSON.parse(storedData);
                setOrgName(parsedUser.organizationName || parsedUser.organization_name || 'My Organization');
                setCreatedBy(parsedUser.email || '');
            }
        } catch (error) {
            console.error("Error loading user data", error);
            setFormError('Failed to load user data. Please refresh and try again.');
        }
    }, []);

    const handleBack = () => { router.back(); };

    const sanitizeLocationName = (value) => value.replace(/\s+/g, '_');

    const getErrorMessage = (data, fallback) => {
        return (
            data?.error?.message ||
            data?.message ||
            data?.details ||
            fallback
        );
    };

    const handleAddInstanceClick = async () => {
        const newErrors = {};
        if (!instanceName.trim()) newErrors.instanceName = "Required field";
        if (!orgName.trim()) newErrors.orgName = "Required field";

        if (Object.keys(newErrors).length > 0) {
            setErrors(newErrors);
            return;
        }

        if (!createdBy.trim()) {
            setFormError('User email not found. Please log in again.');
            return;
        }

        setErrors({});
        setFormError('');
        setCopyError('');
        setHasCopied(false);
        setIsSubmitting(true);

        try {
            const levels = {
                "0": orgName.trim()
            };

            if (level1.trim()) levels["1"] = level1.trim();
            if (level2.trim()) levels["2"] = level2.trim();
            if (level3.trim()) levels["3"] = level3.trim();
            if (level4.trim()) levels["4"] = level4.trim();

            const requestBody = {
                name: instanceName.trim(),
                createdBy: createdBy.trim(),
                location: {
                    levels
                }
            };

            const response = await fetch('/api/v1/devicemgmt/devices', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify(requestBody)
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to create Data Collecting Device (DCD).'));
            }

            if (!data?.device) {
                throw new Error('Device creation succeeded but response is incomplete.');
            }

            setCreatedDeviceResponse(data);
            setShowModal(true);
        } catch (error) {
            console.error('Failed to create Data Collecting Device (DCD):', error);
            setFormError(error.message || 'Failed to create Data Collecting Device (DCD). Please try again.');
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleCloseModal = () => {
        setShowModal(false);
        setCopyError('');
    };

    const handleCopyCommand = async () => {
        try {
            setCopyError('');
            if (!dockerCommand) {
                setCopyError('Docker command not available.');
                return;
            }

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
                    setCopyError('Failed to copy the docker command. Please copy it manually.');
                }
                document.body.removeChild(textArea);
            }
        } catch (err) {
            console.error('Failed to copy text: ', err);
            setCopyError('Failed to copy the docker command. Please copy it manually.');
        }
    };

    const handleContinue = () => {
        if (hasCopied) {
            router.push('/dashboard/Edge-devices');
        }
    };

    return (
        <div className="add-instance-container">
            <div className="add-instance-header">
                <div className="header-left">
                    <button className="back-btn" onClick={handleBack}><ChevronLeft size={24} /></button>
                    <div>
                        <h1 className="page-title">Data Collecting Device Setup</h1>
                        <p className="page-subtitle">Add a new DCD to your infrastructure.</p>
                    </div>
                </div>
                <button className="btn-black-header" onClick={handleAddInstanceClick} disabled={isSubmitting}>
                    {isSubmitting ? 'Adding...' : 'Add DCD'}
                </button>
            </div>

            {formError && (
                <div style={{ color: 'red', marginBottom: '16px', padding: '10px', background: '#ffeeee', borderRadius: '4px' }}>
                    {formError}
                </div>
            )}

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
                                    if (formError) setFormError('');
                                }}
                            />
                        </div>
                    </div>

                    <div className="form-row">
                        <div className="label-col">Created By</div>
                        <div style={{ flex: 1 }}>
                            <input
                                type="text"
                                className="form-input input-locked"
                                value={createdBy}
                                readOnly
                            />
                        </div>
                    </div>
                </div>

                <div className="setup-card">
                    <h3 className="card-title">Location</h3>
                    <p className="card-desc">
                        Define where this Data Collecting Device sits in your organization. Used for data organization and topic routing.
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
                                    setOrgName(sanitizeLocationName(e.target.value));
                                    if (errors.orgName) setErrors({ ...errors, orgName: null });
                                    if (formError) setFormError('');
                                }}
                            />
                            {errors.orgName && <div className="error-text">{errors.orgName}</div>}
                        </div>
                    </div>

                    <div className="location-row">
                        <div className="location-label-col">Level 1</div>
                        <input type="text" className="form-input" placeholder="Your level 1 name" value={level1} onChange={(e) => setLevel1(sanitizeLocationName(e.target.value))} />
                    </div>
                    <div className="location-row">
                        <div className="location-label-col">Level 2</div>
                        <input type="text" className="form-input" placeholder="Your level 2 name" value={level2} onChange={(e) => setLevel2(sanitizeLocationName(e.target.value))} />
                    </div>
                    <div className="location-row">
                        <div className="location-label-col">Level 3</div>
                        <input type="text" className="form-input" placeholder="Your level 3 name" value={level3} onChange={(e) => setLevel3(sanitizeLocationName(e.target.value))} />
                    </div>
                    <div className="location-row">
                        <div className="location-label-col">Level 4</div>
                        <input type="text" className="form-input" placeholder="Your level 4 name" value={level4} onChange={(e) => setLevel4(sanitizeLocationName(e.target.value))} />
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
                        <p className="modal-success-text">Your Data Collecting Device has been created.</p>
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
                        {copyError && <div className="error-text" style={{ marginTop: '10px' }}>{copyError}</div>}
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