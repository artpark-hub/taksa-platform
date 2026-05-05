'use client';

import React from 'react';
import { Plus, Trash2 } from 'lucide-react';
import './General.css';

const General = ({ bridgeConfig, setBridgeConfig }) => {
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
                levels: updatedLevels
            };
        });
    };

    const handleLevelChange = (index, value) => {
        const sanitized = sanitizeLocationName(value);

        updateLevels((levels) => {
            const existing = levels.find((level) => level.index === index);

            if (!existing) {
                return [...levels, { key: `level${index}`, index, label: `Level ${index}`, value: sanitized, isUserAdded: true }];
            }

            return levels.map((level) => (level.index === index ? { ...level, value: sanitized } : level));
        });
    };

    const handleAddLevel = () => {
        updateLevels((levels) => {
            const nextIndex = levels.reduce((max, level) => Math.max(max, level.index), 0) + 1;
            return [...levels, { key: `level${nextIndex}`, index: nextIndex, label: `Level ${nextIndex}`, value: '', isUserAdded: true }];
        });
    };

    const handleRemoveLevel = (index) => {
        updateLevels((levels) => levels.filter((level) => level.index !== index));
    };

    const normalizedLevels = getNormalizedLevels(bridgeConfig);
    const level0 = normalizedLevels.find((level) => level.index === 0) || { key: 'level0', index: 0, label: 'Level 0', value: '' };
    const dynamicLevels = normalizedLevels.filter((level) => level.index > 0);
    const renderedDynamicLevels = dynamicLevels.length > 0
        ? dynamicLevels
        : [{ key: 'level1', index: 1, label: 'Level 1', value: '', isUserAdded: true }];
    const lastLevelIndex = renderedDynamicLevels.length > 0
        ? Math.max(...renderedDynamicLevels.map((level) => level.index))
        : null;
    const canDeleteLevel = (level) => Boolean(level?.isUserAdded) && level.index === lastLevelIndex;

    const handleInputChange = (event) => {
        const { name, value } = event.target;

        setBridgeConfig((prev) => ({
            ...prev,
            [name]: value
        }));
    };

    return (
        <div className="bridge-general-card">
            <div className="bridge-general-card-header">
                <h2>General Information</h2>
                <p>Name and organize this bridge in your hierarchy.</p>
            </div>

            <div className="bridge-general-form">
                <div className="bridge-general-form-row">
                    <label>
                        Name
                        <span>*</span>
                    </label>

                    <input
                        type="text"
                        name="name"
                        value={bridgeConfig.name}
                        placeholder="Enter bridge name"
                        onChange={handleInputChange}
                    />
                </div>

                <div className="bridge-general-form-row">
                    <label>
                        DCD
                        <span>*</span>
                    </label>

                    <input
                        type="text"
                        name="instance"
                        value={bridgeConfig.instance}
                        readOnly
                        disabled
                    />
                </div>

                <div className="bridge-general-form-row" key={level0.key}>
                    <label>
                        Level 0
                        <span>*</span>
                    </label>

                    <input
                        type="text"
                        value={level0.value}
                        readOnly
                    />
                </div>

                {renderedDynamicLevels.map((level) => (
                    <div className="bridge-general-form-row" key={level.key}>
                        <label>
                            {level.label}
                        </label>

                        <div className="bridge-general-level-input-wrap">
                            <input
                                type="text"
                                value={level.value}
                                placeholder={`Your level ${level.index} name`}
                                onChange={(event) => handleLevelChange(level.index, event.target.value)}
                            />

                            {canDeleteLevel(level) && (
                                <button
                                    type="button"
                                    className="bridge-general-level-trash-btn"
                                    onClick={() => handleRemoveLevel(level.index)}
                                    aria-label={`Remove level ${level.index}`}
                                >
                                    <Trash2 size={18} />
                                </button>
                            )}
                        </div>
                    </div>
                ))}

                <button
                    type="button"
                    className="bridge-general-add-level-btn"
                    onClick={handleAddLevel}
                >
                    <Plus size={18} />
                    Add Level {renderedDynamicLevels.length + 1}
                </button>
            </div>
        </div>
    );
};

export default General;