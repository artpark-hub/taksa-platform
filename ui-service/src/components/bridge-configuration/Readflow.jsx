'use client';

import React, { useEffect, useRef, useState } from 'react';
import { ChevronDown, Info, Maximize2 } from 'lucide-react';
import './Readflow.css';

const modbusYaml = `modbus:
  controller: 'tcp://{{ .IP }}:{{ .PORT }}'
  slaveIDs:
    - 1
  timeout: '1s'
  timeBetweenReads: '1s'
  addresses:
    - name: "FirstFlagOfDiscreteInput"
      register: "discrete"
      address: 1
      type: "BIT"
      output: "BOOL"
    - name: "ZeroElementOfInputRegister"
      register: "input"
      address: 0
      type: "UINT16"`;

const injectYaml = `buffer:
  none: {}`;

const Readflow = ({ bridgeConfig, setBridgeConfig }) => {
    const [inputYaml, setInputYaml] = useState(modbusYaml);
    const [injectYamlValue, setInjectYamlValue] = useState(injectYaml);
    const [isEditingInputYaml, setIsEditingInputYaml] = useState(false);
    const [isEditingInjectYaml, setIsEditingInjectYaml] = useState(false);
    const inputYamlRef = useRef(null);
    const injectYamlRef = useRef(null);

    const handleConfigChange = (field, value) => {
        setBridgeConfig((prev) => ({
            ...prev,
            [field]: value
        }));
    };

    useEffect(() => {
        if (isEditingInputYaml) {
            inputYamlRef.current?.focus();
        }
    }, [isEditingInputYaml]);

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

    return (
        <div className="bridge-readflow-container">
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
                            <select
                                value={bridgeConfig.protocol}
                                onChange={(e) => handleConfigChange('protocol', e.target.value)}
                            >
                                <option value="Modbus">Modbus</option>
                                <option value="OPCUA">OPCUA</option>
                            </select>

                            <ChevronDown size={18} className="bridge-readflow-select-icon" />
                        </div>
                    </div>

                    <div className="bridge-readflow-form-row">
                        <label>
                            Data Type
                            <span>*</span>
                        </label>

                        <div className="bridge-readflow-select-row">
                            <div className="bridge-readflow-select-wrapper">
                                <select
                                    value={bridgeConfig.dataType}
                                    onChange={(e) => handleConfigChange('dataType', e.target.value)}
                                >
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

            <div className="bridge-readflow-section-card">
                <h2>Input (YAML)</h2>
                <p>
                    Configure your input in YAML format. See the{' '}
                    <button>Benthos-UMH input documentation.</button>
                </p>

                <div
                    className="bridge-code-editor"
                    onClick={() => setIsEditingInputYaml(true)}
                >
                    {!isEditingInputYaml && <span className="bridge-code-edit-label">Click to edit</span>}
                    <textarea
                        ref={inputYamlRef}
                        className="bridge-code-textarea"
                        value={inputYaml}
                        onChange={(e) => setInputYaml(e.target.value)}
                        onFocus={() => setIsEditingInputYaml(true)}
                        onBlur={() => setIsEditingInputYaml(false)}
                        onKeyDown={(e) => handleYamlKeyDown(e, inputYaml, setInputYaml)}
                        spellCheck={false}
                    />
                    <button className="bridge-code-expand-btn">
                        <Maximize2 size={18} />
                    </button>
                </div>
            </div>

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
                <p>
                    Add custom Benthos resources and configurations. Most users won't need this. See the{' '}
                    <button>Redpanda Connect resources documentation.</button>
                </p>

                <div
                    className="bridge-code-editor large"
                    onClick={() => setIsEditingInjectYaml(true)}
                >
                    {!isEditingInjectYaml && <span className="bridge-code-edit-label">Click to edit</span>}
                    <textarea
                        ref={injectYamlRef}
                        className="bridge-code-textarea"
                        value={injectYamlValue}
                        onChange={(e) => setInjectYamlValue(e.target.value)}
                        onFocus={() => setIsEditingInjectYaml(true)}
                        onBlur={() => setIsEditingInjectYaml(false)}
                        onKeyDown={(e) => handleYamlKeyDown(e, injectYamlValue, setInjectYamlValue)}
                        spellCheck={false}
                    />
                    <button className="bridge-code-expand-btn">
                        <Maximize2 size={18} />
                    </button>
                </div>
            </div>
        </div>
    );
};

export default Readflow;