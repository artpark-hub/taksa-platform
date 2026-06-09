'use client';

import React, { useEffect, useRef, useState } from 'react';
import { ChevronDown, Info, Maximize2, X } from 'lucide-react';
import OpcuaReadflow, { defaultOpcuaYaml } from '../bridge-configuration/OpcuaReadflow';
import './EditOpcuaReadflow.css';

const injectYaml = `buffer:
  none: {}`;

const EditOpcuaReadflow = ({ bridgeConfig, setBridgeConfig }) => {
    const [inputYaml, setInputYaml] = useState(() => bridgeConfig?.readInputYaml || defaultOpcuaYaml);
    const [injectYamlValue, setInjectYamlValue] = useState(() => bridgeConfig?.readRawYamlInject || injectYaml);
    const [isEditingInjectYaml, setIsEditingInjectYaml] = useState(false);
    const [fullscreenEditor, setFullscreenEditor] = useState({ open: false, type: 'input' });
    const injectYamlRef = useRef(null);

    useEffect(() => {
        setBridgeConfig((prev) => ({
            ...prev,
            readInputYaml: inputYaml,
            readRawYamlInject: injectYamlValue,
            readInputType: 'opcua'
        }));
    }, [inputYaml, injectYamlValue, setBridgeConfig]);

    useEffect(() => {
        if (isEditingInjectYaml) {
            injectYamlRef.current?.focus();
        }
    }, [isEditingInjectYaml]);

    const handleYamlKeyDown = (event, value, setValue) => {
        if (event.key !== 'Tab') {
            return;
        }

        event.preventDefault();

        const textarea = event.target;
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const indentation = '  ';
        const updatedValue = `${value.slice(0, start)}${indentation}${value.slice(end)}`;

        setValue(updatedValue);

        requestAnimationFrame(() => {
            textarea.selectionStart = start + indentation.length;
            textarea.selectionEnd = start + indentation.length;
        });
    };

    const openFullscreenEditor = (type) => {
        setFullscreenEditor({ open: true, type });
    };

    const closeFullscreenEditor = () => {
        setFullscreenEditor((prev) => ({ ...prev, open: false }));
    };

    const getModalEditorValue = () => {
        if (fullscreenEditor.type === 'inject') {
            return injectYamlValue;
        }

        return inputYaml;
    };

    const setModalEditorValue = (value) => {
        if (fullscreenEditor.type === 'inject') {
            setInjectYamlValue(value);
            return;
        }

        setInputYaml(value);
    };

    const getModalEditorTitle = () => {
        if (fullscreenEditor.type === 'inject') {
            return 'YAML Inject (Advanced)';
        }

        return 'Input (OPC UA)';
    };

    const getLineNumbers = (value) => {
        const lineCount = Math.max(String(value || '').split('\n').length, 1);
        return Array.from({ length: lineCount }, (_, index) => index + 1).join('\n');
    };

    return (
        <div className="bridge-readflow-container edit-opcua-readflow">
            <div className="bridge-readflow-config-card">
                <div className="bridge-readflow-card-header">
                    <h2>Configuration</h2>
                    <p>How to communicate with your data source</p>
                </div>

                <div className="bridge-readflow-form">
                    <div className="bridge-readflow-form-row">
                        <label>
                            Protocol
                            <span>*</span>
                        </label>

                        <div className="bridge-readflow-select-wrapper">
                            <select value={bridgeConfig.protocol} disabled>
                                <option value="Modbus">Modbus</option>
                                <option value="OPCUA">OPCUA</option>
                            </select>
                        </div>
                    </div>

                    <div className="bridge-readflow-form-row">
                        <label>
                            Data Type
                            <span>*</span>
                        </label>

                        <div className="bridge-readflow-select-row">
                            <div className="bridge-readflow-select-wrapper">
                                <select value={bridgeConfig.dataType} disabled>
                                    <option value="Time Series">Time Series</option>
                                    <option value="Custom Advanced">Custom Advanced</option>
                                </select>

                                <ChevronDown size={18} className="bridge-readflow-select-icon" />
                            </div>

                            <Info size={16} className="bridge-readflow-info-icon" />
                        </div>
                    </div>
                </div>
            </div>

            <OpcuaReadflow
                inputYaml={inputYaml}
                setInputYaml={setInputYaml}
                setBridgeConfig={setBridgeConfig}
                handleYamlKeyDown={handleYamlKeyDown}
                openFullscreenEditor={openFullscreenEditor}
                initialProcessorYaml={bridgeConfig?.readProcessorYaml || ''}
                initialTemplateVariables={bridgeConfig?.templateVariables || []}
            />

            <div className="bridge-readflow-section-card">
                <h2>Output (Unified Namespace)</h2>
                <p>
                    The bridge automatically outputs data to the Unified Namespace, making it available for all connected
                    systems. The output configuration is automatically generated based on your input settings.
                </p>

                <textarea value="(autogenerated)" readOnly />
            </div>

            <div className="bridge-readflow-section-card">
                <h2>YAML Inject (Advanced)</h2>
                <p>Add custom Benthos resources and configurations. Most users won't need this.</p>

                <div
                    className="bridge-code-editor large"
                    onClick={() => setIsEditingInjectYaml(true)}
                >
                    {!isEditingInjectYaml && <span className="bridge-code-edit-label">Click to edit</span>}
                    <textarea
                        ref={injectYamlRef}
                        className="bridge-code-textarea"
                        value={injectYamlValue}
                        onChange={(event) => setInjectYamlValue(event.target.value)}
                        onFocus={() => setIsEditingInjectYaml(true)}
                        onBlur={() => setIsEditingInjectYaml(false)}
                        onKeyDown={(event) => handleYamlKeyDown(event, injectYamlValue, setInjectYamlValue)}
                        spellCheck={false}
                    />
                    <button
                        type="button"
                        className="bridge-code-expand-btn"
                        onClick={(event) => {
                            event.stopPropagation();
                            openFullscreenEditor('inject');
                        }}
                    >
                        <Maximize2 size={18} />
                    </button>
                </div>
            </div>

            {fullscreenEditor.open && (
                <div className="bridge-code-modal-overlay" onClick={closeFullscreenEditor}>
                    <div className="bridge-code-modal" onClick={(event) => event.stopPropagation()}>
                        <div className="bridge-code-modal-header">
                            <h3>{getModalEditorTitle()}</h3>
                            <button
                                type="button"
                                className="bridge-code-modal-close"
                                onClick={closeFullscreenEditor}
                                aria-label="Close fullscreen editor"
                            >
                                <X size={26} />
                            </button>
                        </div>
                        <div className="bridge-code-modal-editor">
                            <pre className="bridge-code-modal-lines">{getLineNumbers(getModalEditorValue())}</pre>
                            <textarea
                                className="bridge-code-modal-textarea"
                                value={getModalEditorValue()}
                                onChange={(event) => setModalEditorValue(event.target.value)}
                                onKeyDown={(event) => handleYamlKeyDown(event, getModalEditorValue(), setModalEditorValue)}
                                spellCheck={false}
                            />
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default EditOpcuaReadflow;
