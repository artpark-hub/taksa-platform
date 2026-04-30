'use client';

import React from 'react';
import { ArrowLeft, ArrowRight, FilePlus, CopyPlus } from 'lucide-react';
import { useRouter } from 'next/navigation';
import './AddBridge.css';

const AddBridge = () => {
    const router = useRouter();

    const handleBack = () => {
        router.push('/bridges');
    };

    const handleCreateFromScratch = () => {
        router.push('/bridges/configure');
    };

    const handleCreateFromExisting = () => {
        // API / navigation logic will be added later
    };

    return (
        <div className="add-bridge-container">
            <div className="add-bridge-header">
                <button className="add-bridge-back-btn" onClick={handleBack}>
                    <ArrowLeft size={22} />
                </button>

                <div>
                    <h1 className="add-bridge-title">Create a New Bridge</h1>
                    <p className="add-bridge-subtitle">
                        Choose how you want to get started with your bridge
                    </p>
                </div>
            </div>

            <div className="bridge-stepper">
                <div className="step-item active">
                    <div className="step-circle">1</div>
                    <p>Choose Starting Point</p>
                </div>

                <div className="step-line half-active"></div>

                <div className="step-item">
                    <div className="step-circle">2</div>
                    <p>Configure Bridge</p>
                </div>

                <div className="step-line"></div>

                <div className="step-item">
                    <div className="step-circle">3</div>
                    <p>Review & Create</p>
                </div>
            </div>

            <div className="bridge-options-wrapper">
                <div className="bridge-option-card">
                    <div className="bridge-option-icon">
                        <FilePlus size={44} />
                    </div>

                    <div className="bridge-option-content">
                        <h2>From Scratch</h2>
                        <p>
                            Build your bridge from the ground up with full control over every configuration detail.
                        </p>

                        <ul>
                            <li>Select a protocol for preconfiguration</li>
                            <li>Complete flexibility to customize all settings</li>
                            <li>Works out of the box – customize only what you need</li>
                        </ul>
                    </div>

                    <button className="bridge-option-btn" onClick={handleCreateFromScratch}>
                        Create From Scratch
                        <ArrowRight size={20} />
                    </button>
                </div>

                <div className="bridge-option-card">
                    <div className="bridge-option-icon">
                        <CopyPlus size={44} />
                    </div>

                    <div className="bridge-option-content">
                        <h2>From Existing Bridge</h2>
                        <p>
                            Start with a proven configuration from an existing bridge or template to save time.
                        </p>

                        <ul>
                            <li>Quick setup with pre-configured settings</li>
                            <li>Use tested configurations from existing bridges</li>
                            <li>Easily modify and adapt to your specific needs</li>
                        </ul>
                    </div>

                    <button className="bridge-option-btn" onClick={handleCreateFromExisting}>
                        Create From Existing Bridge
                        <ArrowRight size={20} />
                    </button>
                </div>
            </div>
        </div>
    );
};

export default AddBridge;