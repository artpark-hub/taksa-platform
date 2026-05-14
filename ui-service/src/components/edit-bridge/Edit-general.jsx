'use client';

import React from 'react';
import { Plus, Trash2 } from 'lucide-react';
import './Edit-general.css';

const EditGeneral = ({ bridgeConfig, setBridgeConfig }) => {
    const sanitizeLocationName = (value) => value.replace(/\s+/g, '_');

    const getNormalizedLevels = (config) => {
        const sourceLevels = Array.isArray(config?.levels) ? config.levels : [];

        const normalized = sourceLevels
            .filter((level) => Number.isInteger(level?.index))
            .sort((a, b) => a.index - b.index)
            .map((level) => ({
                key: String(level?.key || `level${level.index}`),
                index: level.index,
                label: String(level?.label || `Level ${level.index}`),
                value: String(level?.value ?? ''),
                isUserAdded: Boolean(level?.isUserAdded)
            }));

        if (normalized.length === 0) {
            return [{ key: 'level0', index: 0, label: 'Level 0', value: String(config?.level0 || ''), isUserAdded: false }];
        }

        if (!normalized.some((level) => level.index === 0)) {
            normalized.unshift({ key: 'level0', index: 0, label: 'Level 0', value: String(config?.level0 || ''), isUserAdded: false });
        }

        return normalized;
    };

    const updateLevels = (transform) => {
        setBridgeConfig((prev) => {
            const currentLevels = getNormalizedLevels(prev);
            const updatedLevels = transform(currentLevels)
                .sort((a, b) => a.index - b.index)
                .map((level) => ({
                    key: `level${level.index}`,
                    index: level.index,
                    label: `Level ${level.index}`,
                    value: String(level?.value ?? ''),
                    isUserAdded: Boolean(level?.isUserAdded)
                }));

            const level0Value = updatedLevels.find((level) => level.index === 0)?.value || '';

            return {
                ...prev,
                level0: level0Value,
                levels: updatedLevels.filter((level) => level.index !== 0)
            };
        });
    };

    const handleNameChange = (newValue) => {
        setBridgeConfig((prev) => ({
            ...prev,
            name: newValue
        }));
    };

    const handleLevelChange = (index, newValue) => {
        const sanitized = sanitizeLocationName(newValue);

        updateLevels((currentLevels) =>
            currentLevels.map((level) =>
                level.index === index
                    ? { ...level, value: sanitized }
                    : level
            )
        );
    };

    const handleAddLevel = () => {
        updateLevels((currentLevels) => {
            const maxIndex = Math.max(0, ...currentLevels.map((l) => l.index));
            const newLevels = [...currentLevels, {
                key: `level${maxIndex + 1}`,
                index: maxIndex + 1,
                label: `Level ${maxIndex + 1}`,
                value: '',
                isUserAdded: true
            }];

            return newLevels;
        });
    };

    const handleRemoveLevel = (index) => {
        updateLevels((currentLevels) =>
            currentLevels.filter((level) => level.index !== index)
        );
    };

    const levels = getNormalizedLevels(bridgeConfig);

    return (
        <div className="general-container">
            <div className="general-section">
                <h3 className="general-section-title">Bridge Settings</h3>

                <div className="general-form-group">
                    <label className="general-form-label">Bridge Name <span className="required">*</span></label>
                    <input
                        type="text"
                        className="general-form-input"
                        placeholder="Give your bridge a descriptive name"
                        value={bridgeConfig?.name || ''}
                        onChange={(e) => handleNameChange(e.target.value)}
                    />
                </div>

                <div className="general-form-group">
                    <label className="general-form-label">DCD</label>
                    <input
                        type="text"
                        className="general-form-input general-form-input-disabled"
                        value={bridgeConfig?.instance || ''}
                        disabled
                        readOnly
                    />
                </div>
            </div>

            <div className="general-section">
                <h3 className="general-section-title">Location Hierarchy</h3>
                <p className="general-section-desc">
                    Define where this bridge operates in your location hierarchy.
                </p>

                {levels.map((level, idx, arr) => {
                    const isLastLevel = idx === arr.length - 1;
                    const showTrash = level.isUserAdded && isLastLevel;

                    return (
                        <div key={level.key} className="general-location-group">
                            <label className="general-form-label">
                                {level.label}
                                {level.index === 0 && <span className="required">*</span>}
                            </label>

                            <div className="general-location-input-wrapper">
                                <input
                                    type="text"
                                    className="general-form-input"
                                    placeholder={`Enter ${level.label.toLowerCase()}`}
                                    value={level.value}
                                    onChange={(e) => handleLevelChange(level.index, e.target.value)}
                                    disabled={level.index === 0 ? false : false}
                                />

                                {showTrash && (
                                    <button
                                        type="button"
                                        className="general-remove-btn"
                                        onClick={() => handleRemoveLevel(level.index)}
                                        aria-label={`Remove ${level.label}`}
                                    >
                                        <Trash2 size={18} />
                                    </button>
                                )}
                            </div>
                        </div>
                    );
                })}

                <button
                    type="button"
                    className="general-add-level-btn"
                    onClick={handleAddLevel}
                >
                    <Plus size={18} />
                    Add Location Level
                </button>
            </div>
        </div>
    );
};

export default EditGeneral;
