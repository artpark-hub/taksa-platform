'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Download, Maximize2, Plus, Settings2, SlidersHorizontal, Trash2, Upload, X } from 'lucide-react';
import './OpcuaReadflow.css';

const defaultOpcuaYaml = `opcua:
  endpoint: opc.tcp://{{ .IP }}:{{ .PORT }}
  username: ""
  password: ""
  subscribeEnabled: true
  useHeartbeat: true
  nodeIDs:
    - i=84`;

const defaultOpcuaConfig = {
    nodeIDs: 'i=84',
    endpoint: 'opc.tcp://{{ .IP }}:{{ .PORT }}',
    subscribeEnabled: true,
    useHeartbeat: true,
    serverProfile: '',
    username: '',
    password: '',
    securityMode: '',
    securityPolicy: '',
    clientCertificate: '',
    serverCertificateFingerprint: '',
    additionalSettings: []
};

const dataContractOptions = ['_historian', '_raw'];

const defaultNodeRows = [
    { id: 1, nodeId: 'i=84', locationPathSuffix: '', dataContract: '_historian', virtualPath: '', tagName: '', metadata: {} }
];

const getProcessorSwitchCases = (rows = [], defaultDataContract = '_historian') => (rows || [])
    .filter((row) => String(row?.nodeId || '').trim())
    .map((row) => {
        const dataContract = String(row.dataContract || '_historian');
        const lines = [`  case ${JSON.stringify(String(row.nodeId || '').trim())}:`];

        if (String(row.locationPathSuffix || '').trim()) {
            lines.push(`    msg.meta.location_path += ${JSON.stringify(`.${String(row.locationPathSuffix).trim()}`)};`);
        }

        if (dataContract && dataContract !== defaultDataContract) {
            lines.push(`    msg.meta.data_contract = ${JSON.stringify(dataContract)};`);
        }

        if (String(row.virtualPath || '').trim()) {
            lines.push(`    msg.meta.virtual_path = ${JSON.stringify(String(row.virtualPath).trim())};`);
        }

        if (String(row.tagName || '').trim()) {
            lines.push(`    msg.meta.tag_name = ${JSON.stringify(String(row.tagName).trim())};`);
        }

        lines.push('    break;');
        return lines.join('\n');
    })
    .join('\n');

const buildDefaultProcessorCode = ({ dataContract = '_historian', virtualPath = '', rows = defaultNodeRows } = {}) => {
    const switchCases = getProcessorSwitchCases(rows, dataContract);

    return `msg.meta.location_path = "{{ .location_path }}";
msg.meta.data_contract = ${JSON.stringify(dataContract || '_historian')};
msg.meta.tag_name = msg.meta.opcua_tag_name;
msg.payload = msg.payload;
msg.meta.virtual_path = ${JSON.stringify(virtualPath || '')};
msg.meta.timestamp_ms = msg.meta.opcua_source_timestamp;
switch(msg.meta.opcua_attr_nodeid) {
${switchCases ? `${switchCases}\n` : ''}  default:
    break;
}
return msg;`;
};

const defaultProcessorCode = buildDefaultProcessorCode({ rows: defaultNodeRows });

const defaultNodeMappingCsv = `Node ID,Location Path Suffix,Data Contract,Virtual Path,Tag Name
i=84,,_historian,,`;

const defaultConditionCode = `- if: msg.meta.opcua_attr_nodeid === "ns=4;i=6211"
  then: |
    msg.payload = parseFloat(msg.payload) + 273.15;
    msg.meta.tag_name = "CurrentTemperatureKelvin";
    msg.meta.unit = "Kelvin";
    return msg;`;

const defaultDownsamplerSettings = {
    threshold: '0',
    heartbeatInterval: '30',
    heartbeatUnit: 'min'
};

const processorSections = [
    { id: 'defaults', title: 'Default', subtitle: 'Global defaults', count: null },
    { id: 'nodeIds', title: 'Node IDs', subtitle: 'Map OPC-UA nodes', count: 1 },
    { id: 'conditions', title: 'Conditions', subtitle: 'Apply custom rules', count: 1 },
    { id: 'downsampler', title: 'Downsampler', subtitle: 'Reduce duplicate data', count: null }
];


const nodeMappingFields = [
    { key: 'nodeId', label: 'Node ID', required: true },
    { key: 'locationPathSuffix', label: 'Location Path Suffix', required: true },
    { key: 'dataContract', label: 'Data Contract', required: true },
    { key: 'virtualPath', label: 'Virtual Path', required: true },
    { key: 'tagName', label: 'Tag Name', required: true }
];

const conditionFieldOptions = [
    { label: 'Node ID', expression: 'msg.meta.opcua_attr_nodeid' },
    { label: 'Location Path Suffix', expression: 'msg.meta.location_path_suffix' },
    { label: 'Data Contract', expression: 'msg.meta.data_contract' },
    { label: 'Virtual Path', expression: 'msg.meta.virtual_path' },
    { label: 'Tag Name', expression: 'msg.meta.tag_name' }
];

const conditionOperatorOptions = [
    'equals (===)',
    'not equals (!==)',
    'starts with',
    'ends with',
    'contains',
    'greater than (>)',
    'less than (<)',
    'greater or equal (>=)',
    'less or equal (<=)'
];

const getConditionFieldExpression = (field) => conditionFieldOptions.find((option) => option.label === field)?.expression || conditionFieldOptions[0].expression;

const quoteConditionValue = (value) => JSON.stringify(String(value ?? ''));

const getClauseExpression = (clause) => {
    const fieldExpression = getConditionFieldExpression(clause.field);
    const value = quoteConditionValue(clause.value);

    switch (clause.operator) {
        case 'not equals (!==)':
            return `${fieldExpression} !== ${value}`;
        case 'starts with':
            return `String(${fieldExpression}).startsWith(${value})`;
        case 'ends with':
            return `String(${fieldExpression}).endsWith(${value})`;
        case 'contains':
            return `String(${fieldExpression}).includes(${value})`;
        case 'greater than (>)':
            return `${fieldExpression} > ${value}`;
        case 'less than (<)':
            return `${fieldExpression} < ${value}`;
        case 'greater or equal (>=)':
            return `${fieldExpression} >= ${value}`;
        case 'less or equal (<=)':
            return `${fieldExpression} <= ${value}`;
        case 'equals (===)':
        default:
            return `${fieldExpression} === ${value}`;
    }
};

const getConditionExpression = (condition) => {
    const clauses = condition.clauses?.length ? condition.clauses : [{ field: condition.field, operator: condition.operator, value: condition.value }];
    const joiner = condition.joiner === 'OR' ? ' || ' : ' && ';
    return clauses.map(getClauseExpression).join(joiner);
};

const parseClauseExpression = (expression) => {
    const normalized = String(expression || '').trim();
    const field = conditionFieldOptions.find((option) => normalized.startsWith(option.expression)) || conditionFieldOptions[0];
    const operator = conditionOperatorOptions.find((option) => normalized.includes(option.matchText || option.split(' ')[0])) || 'equals (===)';
    const valueMatch = normalized.match(/[=!<>]=?=?\s*["']([^"']*)["']\s*$/);

    return {
        id: 1,
        field: field.label,
        operator: normalized.includes('!==') ? 'not equals (!==)' : operator,
        value: valueMatch?.[1] || ''
    };
};

const parseConditionRowsFromYaml = (conditionYaml) => {
    const text = String(conditionYaml || '').replace(/\r\n/g, '\n').trim();

    if (!text || text === '[]') {
        return [];
    }

    const entries = text.split(/\n(?=-\s+if:)/);
    const rows = entries.map((entry, index) => {
        const ifMatch = entry.match(/^-\s+if:\s*(.*?)(?:\n|$)/);
        const thenMatch = entry.match(/\n\s*then:\s*\|[-]?\n([\s\S]*)$/);

        if (!ifMatch) {
            return null;
        }

        const expression = ifMatch[1].trim();
        const action = thenMatch ? stripBlockIndent(thenMatch[1]) : 'return msg;';

        return {
            id: index + 1,
            joiner: expression.includes(' || ') ? 'OR' : 'AND',
            clauses: [parseClauseExpression(expression)],
            action
        };
    }).filter(Boolean);

    return rows.length ? rows : [{
        id: 1,
        joiner: 'AND',
        clauses: [{ id: 1, field: 'Node ID', operator: 'equals (===)', value: 'ns=4;i=6211' }],
        action: defaultConditionCode.split('then: |\n')[1]?.replace(/^    /gm, '') || 'return msg;'
    }];
};

const escapeCsvValue = (value) => {
    const text = String(value ?? '');
    return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
};

const buildNodeMappingCsv = (rows, columns = []) => {
    const headers = [...nodeMappingFields.map((field) => field.label), ...columns.map((column) => column.label || 'New Column')];

    return [
        headers.map(escapeCsvValue).join(','),
        ...(rows || []).map((row) => [
            row.nodeId,
            row.locationPathSuffix,
            row.dataContract,
            row.virtualPath,
            row.tagName,
            ...columns.map((column) => row.metadata?.[column.id] || '')
        ].map(escapeCsvValue).join(','))
    ].join('\n');
};

const parseCsvLine = (line) => {
    const cells = [];
    let current = '';
    let quoted = false;

    for (let index = 0; index < line.length; index += 1) {
        const char = line[index];
        const nextChar = line[index + 1];

        if (char === '"' && quoted && nextChar === '"') {
            current += '"';
            index += 1;
            continue;
        }

        if (char === '"') {
            quoted = !quoted;
            continue;
        }

        if (char === ',' && !quoted) {
            cells.push(current);
            current = '';
            continue;
        }

        current += char;
    }

    cells.push(current);
    return cells;
};

const normalizeCsvHeader = (value) => String(value || '').trim().toLowerCase();

const getAddressMappingsFromRows = (rows) => (rows || [])
    .filter((row) => String(row?.nodeId || '').trim())
    .map((row) => ({
        Address: String(row.nodeId || '').trim(),
        DataContract: String(row.dataContract || '_historian'),
        LocationPathSuffix: String(row.locationPathSuffix || ''),
        TagName: String(row.tagName || ''),
        VirtualPath: String(row.virtualPath || '')
    }));

const createNodeRowFromMapping = (mapping, index) => ({
    id: index + 1,
    nodeId: String(mapping?.Address || mapping?.nodeId || '').trim(),
    locationPathSuffix: String(mapping?.LocationPathSuffix || mapping?.locationPathSuffix || ''),
    dataContract: String(mapping?.DataContract || mapping?.dataContract || '_historian'),
    virtualPath: String(mapping?.VirtualPath || mapping?.virtualPath || ''),
    tagName: String(mapping?.TagName || mapping?.tagName || ''),
    metadata: {}
});

const parseAddressMappingsVariable = (variables) => {
    const addressVariable = (variables || []).find((variable) => String(variable?.label || '').trim() === 'AddressMappings');

    if (!addressVariable) {
        return [];
    }

    const rawValue = addressVariable.value;

    if (Array.isArray(rawValue)) {
        return rawValue;
    }

    if (typeof rawValue !== 'string') {
        return [];
    }

    try {
        const parsed = JSON.parse(rawValue);
        return Array.isArray(parsed) ? parsed : [];
    } catch {
        return [];
    }
};

const stripBlockIndent = (value) => {
    const lines = String(value || '').replace(/\r\n/g, '\n').replace(/^\n+|\n+$/g, '').split('\n');
    const indents = lines
        .filter((line) => line.trim())
        .map((line) => line.match(/^\s*/)?.[0].length || 0);
    const minIndent = indents.length ? Math.min(...indents) : 0;

    return lines.map((line) => line.slice(Math.min(minIndent, line.length))).join('\n').replace(/\s+$/g, '');
};

const stripTagProcessorRoot = (yaml) => {
    const text = String(yaml || '').replace(/\r\n/g, '\n').trim();

    if (!text.startsWith('tag_processor:')) {
        return text;
    }

    return stripBlockIndent(text.split('\n').slice(1).join('\n'));
};

const escapeRegExp = (value) => String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const extractYamlSection = (yaml, key, nextKeys = []) => {
    const text = stripTagProcessorRoot(yaml);
    const escapedKey = escapeRegExp(key);
    const escapedNextKeys = nextKeys.map(escapeRegExp).join('|');
    const nextPattern = escapedNextKeys ? `(?=\\n\\s*(?:${escapedNextKeys}):|$)` : '$';
    const blockPattern = new RegExp(`(?:^|\\n)\\s*${escapedKey}:\\s*(?:\\|[-]?)?\\n([\\s\\S]*?)${nextPattern}`);
    const inlinePattern = new RegExp(`(?:^|\\n)\\s*${escapedKey}:\\s*([^\\n]+)`);
    const blockMatch = text.match(blockPattern);

    if (blockMatch) {
        return stripBlockIndent(blockMatch[1]);
    }

    const inlineMatch = text.match(inlinePattern);
    return inlineMatch ? inlineMatch[1].trim() : null;
};

const getDefaultSettingsFromCode = (code) => {
    const dataContractMatch = String(code || '').match(/msg\.meta\.data_contract\s*=\s*["']([^"']*)["']/);
    const virtualPathMatch = String(code || '').match(/msg\.meta\.virtual_path\s*=\s*["']([^"']*)["']/);

    return {
        dataContract: dataContractMatch?.[1] || '_historian',
        virtualPath: virtualPathMatch?.[1] || ''
    };
};

const parseOpcuaProcessorYaml = (yaml) => {
    const defaultCode = extractYamlSection(yaml, 'defaults') || defaultProcessorCode;
    const conditionCode = extractYamlSection(yaml, 'conditions', ['defaults']) || defaultConditionCode;

    return {
        defaultCode,
        conditionCode,
        defaultSettings: getDefaultSettingsFromCode(defaultCode)
    };
};

const indentBlock = (value, spaces = 4) => String(value || '')
    .split('\n')
    .map((line) => `${' '.repeat(spaces)}${line}`)
    .join('\n');

const formatOpcuaConditionsYaml = (value) => {
    const normalized = String(value || '').trim();
    return normalized ? indentBlock(normalized, 4) : '    []';
};

const buildOpcuaProcessorYaml = ({ defaultCode, conditionCode }) => `tag_processor:
  advancedProcessing: return msg;
  conditions:
${formatOpcuaConditionsYaml(conditionCode)}
  defaults: |-
${indentBlock(defaultCode, 4)}`;

const advancedFields = [
    {
        key: 'serverProfile',
        yamlKey: 'serverProfile',
        label: 'Server Profile',
        description: 'Server profile for performance tuning. Leave empty for automatic detection (recommended). Examples: "", "auto", "high-performance", "ignition", "kepware", "siemens-s7-1200", "siemens-s7-1500", "prosys"'
    },
    {
        key: 'username',
        yamlKey: 'username',
        label: 'Username',
        description: 'The username for authentication. Examples: "", "admin", "opcuser"'
    },
    {
        key: 'password',
        yamlKey: 'password',
        label: 'Password',
        description: 'The password for authentication. Examples: "", "password123"',
        type: 'password'
    },
    {
        key: 'securityMode',
        yamlKey: 'securityMode',
        label: 'Security Mode',
        description: 'The security mode to use. Leave empty to connect without encryption (only if server supports None). Examples: "", "None", "Sign", "SignAndEncrypt"'
    },
    {
        key: 'securityPolicy',
        yamlKey: 'securityPolicy',
        label: 'Security Policy',
        description: 'The security policy to use. Leave empty to connect without encryption (only if server supports None). Examples: "", "None", "Basic256", "Basic256Sha256"'
    },
    {
        key: 'clientCertificate',
        yamlKey: 'clientCertificate',
        label: 'Client Certificate',
        description: 'The client certificate to use, base64-encoded.'
    },
    {
        key: 'serverCertificateFingerprint',
        yamlKey: 'serverCertificateFingerprint',
        label: 'Server Certificate Fingerprint',
        description: 'The server certificate fingerprint to verify, SHA3-512 hash.'
    }
];

const yamlToConfigKey = advancedFields.reduce((acc, field) => {
    acc[field.yamlKey] = field.key;
    return acc;
}, {});

const hiddenYamlKeys = new Set(['endpoint', 'subscribeEnabled', 'useHeartbeat']);

const stripInlineComment = (value) => {
    const text = String(value ?? '').trim();
    const commentIndex = text.indexOf(' #');
    return commentIndex >= 0 ? text.slice(0, commentIndex).trim() : text;
};

const stripYamlValue = (value) => {
    const text = stripInlineComment(value);

    if ((text.startsWith('"') && text.endsWith('"')) || (text.startsWith("'") && text.endsWith("'"))) {
        return text.slice(1, -1);
    }

    return text;
};

const parseInlineNodeIDs = (value) => {
    const normalized = stripYamlValue(value);

    if (!normalized || normalized === '[]') {
        return '';
    }

    if (normalized.startsWith('[') && normalized.endsWith(']')) {
        return normalized
            .slice(1, -1)
            .split(',')
            .map((item) => stripYamlValue(item))
            .filter(Boolean)
            .join('\n');
    }

    return normalized;
};

const getNodeIDLines = (value) => String(value || '')
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);

const getInitialNodeRows = (inputYaml, variables) => {
    const addressMappings = parseAddressMappingsVariable(variables);

    if (addressMappings.length > 0) {
        return addressMappings.map(createNodeRowFromMapping);
    }

    const parsedInput = parseOpcuaYaml(inputYaml);
    const nodeIDs = getNodeIDLines(parsedInput.nodeIDs);

    if (nodeIDs.length === 0) {
        return defaultNodeRows.map((row) => ({ ...row, metadata: { ...(row.metadata || {}) } }));
    }

    return nodeIDs.map((nodeId, index) => createNodeRowFromMapping({ Address: nodeId, DataContract: '_historian' }, index));
};

const shouldQuoteScalar = (value) => {
    const text = String(value ?? '');

    return (
        !text ||
        text !== text.trim() ||
        text.includes(' #') ||
        text.includes(': ') ||
        text.includes('\n') ||
        text.startsWith('-') ||
        text.startsWith('{') ||
        text.startsWith('[')
    );
};

const formatYamlScalar = (value) => {
    const text = String(value ?? '');
    const lower = text.toLowerCase();

    if (lower === 'true' || lower === 'false') {
        return lower;
    }

    if (shouldQuoteScalar(text)) {
        return JSON.stringify(text);
    }

    return text;
};

const appendYamlSetting = (lines, key, value) => {
    const text = String(value ?? '');

    if (text.includes('\n')) {
        lines.push(`  ${key}: |`);
        text.split('\n').forEach((line) => {
            lines.push(`    ${line}`);
        });
        return;
    }

    lines.push(`  ${key}: ${formatYamlScalar(text)}`);
};

const buildOpcuaYaml = (config) => {
    const nodeIDs = getNodeIDLines(config.nodeIDs);
    const lines = ['opcua:'];

    lines.push(`  endpoint: ${formatYamlScalar(config.endpoint || 'opc.tcp://{{ .IP }}:{{ .PORT }}')}`);
    lines.push(`  username: ${formatYamlScalar(config.username || '')}`);
    lines.push(`  password: ${formatYamlScalar(config.password || '')}`);
    lines.push(`  subscribeEnabled: ${config.subscribeEnabled === false ? 'false' : 'true'}`);
    lines.push(`  useHeartbeat: ${config.useHeartbeat === false ? 'false' : 'true'}`);

    if (nodeIDs.length > 0) {
        lines.push('  nodeIDs:');
        nodeIDs.forEach((nodeId) => {
            lines.push(`    - ${formatYamlScalar(nodeId)}`);
        });
    } else {
        lines.push('  nodeIDs: []');
    }

    advancedFields.forEach((field) => {
        if (field.key === 'username' || field.key === 'password') {
            return;
        }

        const value = config[field.key];

        if (String(value || '').trim()) {
            appendYamlSetting(lines, field.yamlKey, value);
        }
    });

    (config.additionalSettings || []).forEach((setting) => {
        const key = String(setting?.key || '').trim();

        if (!key) {
            return;
        }

        appendYamlSetting(lines, key, setting?.value || '');
    });

    return lines.join('\n');
};

const parseOpcuaYaml = (yaml) => {
    const config = {
        ...defaultOpcuaConfig,
        additionalSettings: []
    };
    const lines = String(yaml || '').replace(/\r\n/g, '\n').split('\n');

    for (let index = 0; index < lines.length; index += 1) {
        const rawLine = lines[index];
        const trimmed = rawLine.trim();

        if (!trimmed || trimmed === 'opcua:' || trimmed.startsWith('#') || !rawLine.startsWith('  ')) {
            continue;
        }

        const separatorIndex = trimmed.indexOf(':');

        if (separatorIndex < 0) {
            continue;
        }

        const yamlKey = trimmed.slice(0, separatorIndex).trim();
        const rawValue = trimmed.slice(separatorIndex + 1).trim();
        let value = stripYamlValue(rawValue);

        if (yamlKey === 'nodeIDs') {
            const nodeIDs = [];

            if (rawValue && rawValue !== '[]') {
                config.nodeIDs = parseInlineNodeIDs(rawValue);
                continue;
            }

            while (index + 1 < lines.length) {
                const nextLine = lines[index + 1];
                const nextTrimmed = nextLine.trim();

                if (!nextLine.startsWith('    ') || !nextTrimmed.startsWith('- ')) {
                    break;
                }

                index += 1;
                nodeIDs.push(stripYamlValue(nextTrimmed.slice(2)));
            }

            config.nodeIDs = nodeIDs.join('\n');
            continue;
        }

        if (rawValue === '|' || rawValue === '|-') {
            const blockLines = [];

            while (index + 1 < lines.length && lines[index + 1].startsWith('    ')) {
                index += 1;
                blockLines.push(lines[index].slice(4));
            }

            value = blockLines.join('\n').replace(/\s+$/, '');
        }

        if (yamlKey === 'endpoint') {
            config.endpoint = value;
            continue;
        }

        if (yamlKey === 'subscribeEnabled') {
            config.subscribeEnabled = value !== 'false';
            continue;
        }

        if (yamlKey === 'useHeartbeat') {
            config.useHeartbeat = value !== 'false';
            continue;
        }

        if (hiddenYamlKeys.has(yamlKey)) {
            continue;
        }

        const configKey = yamlToConfigKey[yamlKey];

        if (configKey) {
            config[configKey] = value;
            continue;
        }

        config.additionalSettings.push({ key: yamlKey, value });
    }

    return config;
};

const getLineNumbers = (value) => {
    const lineCount = Math.max(String(value || '').split('\n').length, 1);
    return Array.from({ length: lineCount }, (_, index) => index + 1).join('\n');
};

const OpcuaReadflow = ({
    inputYaml,
    setInputYaml,
    setBridgeConfig,
    handleYamlKeyDown,
    openFullscreenEditor,
    initialProcessorYaml = '',
    initialTemplateVariables = []
}) => {
    const initialProcessor = useMemo(() => parseOpcuaProcessorYaml(initialProcessorYaml), []);
    const initialRows = useMemo(() => getInitialNodeRows(inputYaml, initialTemplateVariables), []);
    const hasInitialProcessorYaml = String(initialProcessorYaml || '').trim().length > 0;
    const didSkipInitialProcessorSyncRef = useRef(false);
    const [viewMode, setViewMode] = useState('Visual');
    const [activeSection, setActiveSection] = useState('standard');
    const [isEditingCode, setIsEditingCode] = useState(false);
    const [additionalDrafts, setAdditionalDrafts] = useState(null);
    const [processorView, setProcessorView] = useState('Visual');
    const [processorSection, setProcessorSection] = useState('defaults');
    const [isEditingProcessorCode, setIsEditingProcessorCode] = useState(false);
    const [fullscreenProcessorCode, setFullscreenProcessorCode] = useState(false);
    const [defaultCode, setDefaultCode] = useState(initialProcessor.defaultCode);
    const [nodeMappingCsv, setNodeMappingCsv] = useState(() => buildNodeMappingCsv(initialRows));
    const [conditionCode, setConditionCode] = useState(initialProcessor.conditionCode);
    const [downsamplerSettings, setDownsamplerSettings] = useState(defaultDownsamplerSettings);
    const [defaultSettings, setDefaultSettings] = useState(initialProcessor.defaultSettings);
    const [nodeSearch, setNodeSearch] = useState('');
    const [nodeImportError, setNodeImportError] = useState('');
    const [metadataColumns, setMetadataColumns] = useState([]);
    const [hasNodeMappingChanges, setHasNodeMappingChanges] = useState(false);
    const importCsvRef = useRef(null);
    const [nodeRows, setNodeRows] = useState(initialRows);
    const [conditionRows, setConditionRows] = useState(() => parseConditionRowsFromYaml(initialProcessor.conditionCode));
    const parsedConfig = useMemo(() => parseOpcuaYaml(inputYaml), [inputYaml]);
    const additionalSettings = additionalDrafts || parsedConfig.additionalSettings;
    const displayConfig = {
        ...parsedConfig,
        additionalSettings
    };
    const additionalCount = additionalSettings.length;

    const activeProcessorSection = processorSections.find((section) => section.id === processorSection) || processorSections[0];
    const isDownsamplerActive = processorSection === 'downsampler';

    useEffect(() => {
        if (isDownsamplerActive && processorView === 'Code') {
            setProcessorView('Visual');
        }
    }, [isDownsamplerActive, processorView]);

    useEffect(() => {
        if (!setBridgeConfig) {
            return;
        }

        if (hasInitialProcessorYaml && !didSkipInitialProcessorSyncRef.current) {
            didSkipInitialProcessorSyncRef.current = true;
            return;
        }

        const processorYaml = buildOpcuaProcessorYaml({
            defaultCode,
            conditionCode
        });
        const addressMappings = getAddressMappingsFromRows(nodeRows);

        setBridgeConfig((prev) => ({
            ...prev,
            readProcessorType: 'tag_processor',
            readProcessorYaml: processorYaml,
            readTemplateVariables: hasNodeMappingChanges
                ? [{ label: 'AddressMappings', value: JSON.stringify(addressMappings) }]
                : []
        }));
    }, [conditionCode, defaultCode, hasInitialProcessorYaml, hasNodeMappingChanges, nodeRows, setBridgeConfig]);

    const updateDefaultSetting = (field, value) => {
        const nextSettings = {
            ...defaultSettings,
            [field]: value
        };
        setDefaultSettings(nextSettings);
        setDefaultCode(buildDefaultProcessorCode({ ...nextSettings, rows: nodeRows }));
    };

    const syncInputNodeIDsFromRows = (rows) => {
        const nodeIDs = getAddressMappingsFromRows(rows)
            .map((mapping) => mapping.Address)
            .join('\n');

        setInputYaml(buildOpcuaYaml({
            ...parsedConfig,
            additionalSettings,
            nodeIDs
        }));
    };

    const syncNodeMappingCsv = (rows, columns = metadataColumns, { markChanged = true } = {}) => {
        const headers = [...nodeMappingFields.map((field) => field.label), ...columns.map((column) => column.label || 'New Column')];
        const csv = [
            headers.map(escapeCsvValue).join(','),
            ...rows.map((row) => [
                row.nodeId,
                row.locationPathSuffix,
                row.dataContract,
                row.virtualPath,
                row.tagName,
                ...columns.map((column) => row.metadata?.[column.id] || '')
            ].map(escapeCsvValue).join(','))
        ].join('\n');

        if (markChanged) {
            setHasNodeMappingChanges(true);
        }

        setDefaultCode(buildDefaultProcessorCode({ ...defaultSettings, rows }));
        setNodeMappingCsv(csv);
        syncInputNodeIDsFromRows(rows);
    };

    const updateNodeRow = (id, field, value) => {
        const rows = nodeRows.map((row) => (row.id === id ? { ...row, [field]: value } : row));
        setNodeRows(rows);
        syncNodeMappingCsv(rows);
    };

    const updateNodeMetadata = (id, columnId, value) => {
        const rows = nodeRows.map((row) => (
            row.id === id
                ? { ...row, metadata: { ...(row.metadata || {}), [columnId]: value } }
                : row
        ));
        setNodeRows(rows);
        syncNodeMappingCsv(rows);
    };

    const addNodeRow = () => {
        const nextId = Math.max(...nodeRows.map((row) => row.id), 0) + 1;
        const rows = [...nodeRows, { id: nextId, nodeId: '', locationPathSuffix: '', dataContract: '_historian', virtualPath: '', tagName: '', metadata: {} }];
        setNodeRows(rows);
        syncNodeMappingCsv(rows);
    };

    const removeNodeRow = (id) => {
        const rows = nodeRows.filter((row) => row.id !== id);
        setNodeRows(rows);
        syncNodeMappingCsv(rows);
    };

    const addMetadataColumn = () => {
        const nextId = `metadata_${Date.now()}`;
        const columns = [...metadataColumns, { id: nextId, label: 'New Column' }];
        setMetadataColumns(columns);
        syncNodeMappingCsv(nodeRows, columns);
    };

    const updateMetadataColumn = (id, label) => {
        const columns = metadataColumns.map((column) => (column.id === id ? { ...column, label } : column));
        setMetadataColumns(columns);
        syncNodeMappingCsv(nodeRows, columns);
    };

    const removeMetadataColumn = (id) => {
        const columns = metadataColumns.filter((column) => column.id !== id);
        const rows = nodeRows.map((row) => {
            const metadata = { ...(row.metadata || {}) };
            delete metadata[id];
            return { ...row, metadata };
        });
        setMetadataColumns(columns);
        setNodeRows(rows);
        syncNodeMappingCsv(rows, columns);
    };

    const exportNodeMappingCsv = () => {
        const blob = new Blob([nodeMappingCsv], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = 'opcua-node-mapping.csv';
        link.click();
        URL.revokeObjectURL(url);
    };

    const parseNodeMappingCsvContent = (csvContent) => {
        const lines = String(csvContent || '').split(/\r?\n/).filter((line) => line.trim());
        const fallbackRow = { id: 1, nodeId: '', locationPathSuffix: '', dataContract: '_historian', virtualPath: '', tagName: '', metadata: {} };

        const requiredHeaders = nodeMappingFields.map((field) => field.label);

        if (lines.length === 0) {
            return {
                error: `Missing columns in CSV file: ${requiredHeaders.join(', ')}`,
                columns: [],
                rows: [fallbackRow]
            };
        }

        const headers = parseCsvLine(lines[0]);
        const headerLookup = new Map(headers.map((header, index) => [normalizeCsvHeader(header), index]));
        const missingColumns = requiredHeaders.filter((label) => !headerLookup.has(normalizeCsvHeader(label)));

        if (missingColumns.length) {
            return {
                error: `Missing columns in CSV file: ${missingColumns.join(', ')}`,
                columns: [],
                rows: [fallbackRow]
            };
        }

        const stamp = Date.now();
        const requiredHeaderSet = new Set(requiredHeaders.map(normalizeCsvHeader));
        const extraHeaders = headers
            .map((label, index) => ({ label, index }))
            .filter(({ label }) => label && !requiredHeaderSet.has(normalizeCsvHeader(label)));
        const columns = extraHeaders.map(({ label }, index) => ({
            id: `metadata_import_${stamp}_${index}`,
            label: label || 'New Column'
        }));
        const getCell = (cells, label) => cells[headerLookup.get(normalizeCsvHeader(label))] || '';

        const importedRows = lines.slice(1).map((line, index) => {
            const cells = parseCsvLine(line);
            const metadata = {};

            columns.forEach((column, columnIndex) => {
                metadata[column.id] = cells[extraHeaders[columnIndex].index] || '';
            });

            return {
                id: index + 1,
                nodeId: getCell(cells, 'Node ID'),
                locationPathSuffix: getCell(cells, 'Location Path Suffix'),
                dataContract: getCell(cells, 'Data Contract') || '_historian',
                virtualPath: getCell(cells, 'Virtual Path'),
                tagName: getCell(cells, 'Tag Name'),
                metadata
            };
        });

        return {
            columns,
            rows: importedRows.length ? importedRows : [fallbackRow]
        };
    };

    const importNodeMappingCsv = (event) => {
        const file = event.target.files?.[0];
        if (!file) {
            return;
        }

        const reader = new FileReader();
        reader.onload = () => {
            const content = String(reader.result || '');
            const parsed = parseNodeMappingCsvContent(content);

            if (parsed.error) {
                setNodeImportError(parsed.error);
                return;
            }

            setNodeImportError('');
            setMetadataColumns(parsed.columns);
            setNodeRows(parsed.rows);
            syncNodeMappingCsv(parsed.rows, parsed.columns);
        };
        reader.readAsText(file);
        event.target.value = '';
    };

    const visibleNodeRows = nodeRows.filter((row) => !nodeSearch.trim() || row.nodeId.toLowerCase().includes(nodeSearch.trim().toLowerCase()));

    const syncConditionCode = (rows) => {
        const yaml = rows.map((row) => `- if: ${getConditionExpression(row)}
  then: |
${indentBlock(row.action || 'return msg;', 4)}`).join('\n');
        setConditionCode(yaml);
    };

    const updateConditionAction = (id, value) => {
        const rows = conditionRows.map((row) => (row.id === id ? { ...row, action: value } : row));
        setConditionRows(rows);
        syncConditionCode(rows);
    };

    const updateConditionClause = (conditionId, clauseId, field, value) => {
        const rows = conditionRows.map((row) => (
            row.id === conditionId
                ? { ...row, clauses: row.clauses.map((clause) => (clause.id === clauseId ? { ...clause, [field]: value } : clause)) }
                : row
        ));
        setConditionRows(rows);
        syncConditionCode(rows);
    };

    const addConditionClause = (conditionId) => {
        const rows = conditionRows.map((row) => {
            if (row.id !== conditionId || row.clauses.length >= 2) {
                return row;
            }
            const nextId = Math.max(...row.clauses.map((clause) => clause.id), 0) + 1;
            return { ...row, clauses: [...row.clauses, { id: nextId, field: 'Node ID', operator: 'equals (===)', value: '' }] };
        });
        setConditionRows(rows);
        syncConditionCode(rows);
    };

    const removeConditionClause = (conditionId, clauseId) => {
        const rows = conditionRows.map((row) => (
            row.id === conditionId
                ? { ...row, clauses: row.clauses.filter((clause) => clause.id !== clauseId) }
                : row
        ));
        setConditionRows(rows);
        syncConditionCode(rows);
    };

    const toggleConditionJoiner = (conditionId) => {
        const rows = conditionRows.map((row) => (
            row.id === conditionId ? { ...row, joiner: row.joiner === 'AND' ? 'OR' : 'AND' } : row
        ));
        setConditionRows(rows);
        syncConditionCode(rows);
    };

    const addConditionRow = () => {
        const nextId = Math.max(...conditionRows.map((row) => row.id), 0) + 1;
        const rows = [...conditionRows, {
            id: nextId,
            joiner: 'AND',
            clauses: [{ id: 1, field: 'Node ID', operator: 'equals (===)', value: '' }],
            action: 'return msg;'
        }];
        setConditionRows(rows);
        syncConditionCode(rows);
    };

    const removeConditionRow = (id) => {
        const rows = conditionRows.filter((row) => row.id !== id);
        setConditionRows(rows);
        syncConditionCode(rows);
    };

    const updateDownsamplerSetting = (field, value) => {
        setDownsamplerSettings((prev) => ({
            ...prev,
            [field]: value
        }));
    };

    const getProcessorCodeValue = () => {
        if (processorSection === 'nodeIds') {
            return nodeMappingCsv;
        }

        if (processorSection === 'conditions') {
            return conditionCode;
        }

        return defaultCode;
    };

    const setProcessorCodeValue = (value) => {
        if (processorSection === 'nodeIds') {
            setNodeMappingCsv(value);
            return;
        }

        if (processorSection === 'conditions') {
            setConditionCode(value);
            return;
        }

        setDefaultCode(value);
    };

    const updateConfig = (patch) => {
        setInputYaml(buildOpcuaYaml({
            ...parsedConfig,
            additionalSettings,
            ...patch
        }));
    };

    const updateAdditionalSettings = (settings) => {
        setAdditionalDrafts(settings);
        setInputYaml(buildOpcuaYaml({
            ...parsedConfig,
            additionalSettings: settings
        }));
    };

    const addAdditionalSetting = () => {
        setActiveSection('additional');
        setAdditionalDrafts([
            ...additionalSettings,
            { key: '', value: '' }
        ]);
    };

    const updateAdditionalSetting = (index, field, value) => {
        const nextSettings = additionalSettings.map((setting, settingIndex) => (
            settingIndex === index ? { ...setting, [field]: value } : setting
        ));
        updateAdditionalSettings(nextSettings);
    };

    const removeAdditionalSetting = (index) => {
        const nextSettings = additionalSettings.filter((_, settingIndex) => settingIndex !== index);
        updateAdditionalSettings(nextSettings);
    };

    const handleCodeChange = (value) => {
        setAdditionalDrafts(null);
        setInputYaml(value);
    };

    const renderStandardAdvancedSettings = () => (
        <div className="opcua-settings-panel">
            <div className="opcua-field-row">
                <div>
                    <label>Node IDs to Subscribe</label>
                    <p>Managed as OPC UA node identifiers. Add one per line or separate with commas.</p>
                </div>
                <textarea
                    value={displayConfig.nodeIDs}
                    onChange={(event) => updateConfig({ nodeIDs: event.target.value })}
                    placeholder="{{ .nodeIDs }}"
                    rows={3}
                />
            </div>

            <div className="opcua-advanced-list">
                {advancedFields.map((field) => (
                    <div className="opcua-field-row" key={field.key}>
                        <div>
                            <label>{field.label}</label>
                            <p>{field.description}</p>
                        </div>
                        <input
                            type={field.type || 'text'}
                            value={displayConfig[field.key]}
                            onChange={(event) => updateConfig({ [field.key]: event.target.value })}
                        />
                    </div>
                ))}
            </div>
        </div>
    );

    const renderAdditionalSettings = () => (
        <div className="opcua-settings-panel">
            {additionalSettings.length === 0 ? (
                <div className="opcua-additional-empty">
                    <p>No additional settings. Click "Add Setting" to create one.</p>
                </div>
            ) : (
                <div className="opcua-additional-list">
                    {additionalSettings.map((setting, index) => (
                        <div className="opcua-additional-row" key={`setting-${index}`}>
                            <div className="opcua-additional-field">
                                <label>Key</label>
                                <input
                                    type="text"
                                    value={setting.key}
                                    onChange={(event) => updateAdditionalSetting(index, 'key', event.target.value)}
                                    placeholder="field.name"
                                />
                                <small>YAML field name</small>
                            </div>

                            <div className="opcua-additional-field">
                                <label>Value</label>
                                <input
                                    type="text"
                                    value={setting.value}
                                    onChange={(event) => updateAdditionalSetting(index, 'value', event.target.value)}
                                    placeholder="value or YAML content"
                                />
                                <small>Value or YAML content</small>
                            </div>

                            <button
                                type="button"
                                className="opcua-delete-setting-btn"
                                onClick={() => removeAdditionalSetting(index)}
                                aria-label="Delete additional setting"
                            >
                                <Trash2 size={18} />
                            </button>
                        </div>
                    ))}
                </div>
            )}

            <button type="button" className="opcua-add-setting-btn" onClick={addAdditionalSetting}>
                <Plus size={18} />
                Add Setting
            </button>
        </div>
    );

    const renderProcessorDefaults = () => (
        <div className="opcua-settings-panel">
            <div className="opcua-field-row">
                <div>
                    <label>Location Path</label>
                    <p>Managed in the Instance and Bridge settings.</p>
                </div>
                <input type="text" value="{{ .location_path }}" readOnly />
            </div>

            <div className="opcua-field-row">
                <div>
                    <label>Data Contract</label>
                    <p>Select the contract written to OPC UA message metadata.</p>
                </div>
                <select
                    className="opcua-select-input"
                    value={defaultSettings.dataContract}
                    onChange={(event) => updateDefaultSetting('dataContract', event.target.value)}
                >
                    {dataContractOptions.map((option) => (
                        <option key={option} value={option}>{option}</option>
                    ))}
                </select>
            </div>

            <div className="opcua-field-row">
                <div>
                    <label>Virtual Path</label>
                    <p>Optional path override for tag output.</p>
                </div>
                <input
                    type="text"
                    value={defaultSettings.virtualPath}
                    onChange={(event) => updateDefaultSetting('virtualPath', event.target.value)}
                    placeholder="Optional virtual path"
                />
            </div>
        </div>
    );

    const renderProcessorNodeIds = () => (
        <div className="opcua-processor-panel">
            <p className="opcua-processor-note">OPC-UA Node ID formats: i=84 (namespace 0 implied), ns=2;s=DeviceName (string), ns=3;i=1001 (numeric)</p>
            <div className="opcua-node-toolbar">
                <input
                    type="search"
                    value={nodeSearch}
                    onChange={(event) => setNodeSearch(event.target.value)}
                    placeholder="Search Node IDs..."
                    aria-label="Search Node IDs"
                />
                <div className="opcua-node-actions">
                    <button type="button" className="opcua-secondary-btn" onClick={exportNodeMappingCsv}>
                        <Download size={16} />
                        Export CSV
                    </button>
                    <button type="button" className="opcua-secondary-btn" onClick={() => importCsvRef.current?.click()}>
                        <Upload size={16} />
                        Import CSV
                    </button>
                    <input
                        ref={importCsvRef}
                        type="file"
                        accept=".csv,text/csv"
                        className="opcua-hidden-file-input"
                        onChange={importNodeMappingCsv}
                    />
                </div>
            </div>
            {nodeImportError && <div className="opcua-node-import-error">{nodeImportError}</div>}
            <div className="opcua-node-grid-shell">
                <div
                    className="opcua-node-table-wrap"
                    style={{ maxHeight: `calc(${(visibleNodeRows.length + 1) * 3.25}rem + 2px)` }}
                >
                    <table className="opcua-node-table">
                        <thead>
                            <tr>
                                {nodeMappingFields.map((field) => (
                                    <th key={field.key}>{field.label} {field.required && <span>*</span>}</th>
                                ))}
                                {metadataColumns.map((column) => (
                                    <th key={column.id} className="opcua-metadata-heading">
                                        <input
                                            value={column.label}
                                            onChange={(event) => updateMetadataColumn(column.id, event.target.value)}
                                            aria-label="Metadata column name"
                                        />
                                        <button type="button" className="opcua-icon-btn" onClick={() => removeMetadataColumn(column.id)} aria-label="Delete metadata column">
                                            <Trash2 size={16} />
                                        </button>
                                    </th>
                                ))}
                                <th></th>
                            </tr>
                        </thead>
                        <tbody>
                            {visibleNodeRows.map((row) => (
                                <tr key={row.id}>
                                    <td><input value={row.nodeId} onChange={(event) => updateNodeRow(row.id, 'nodeId', event.target.value)} /></td>
                                    <td><input value={row.locationPathSuffix} onChange={(event) => updateNodeRow(row.id, 'locationPathSuffix', event.target.value)} /></td>
                                    <td>
                                        <select value={row.dataContract} onChange={(event) => updateNodeRow(row.id, 'dataContract', event.target.value)}>
                                            {dataContractOptions.map((option) => <option key={option} value={option}>{option}</option>)}
                                        </select>
                                    </td>
                                    <td><input value={row.virtualPath} onChange={(event) => updateNodeRow(row.id, 'virtualPath', event.target.value)} /></td>
                                    <td><input value={row.tagName} onChange={(event) => updateNodeRow(row.id, 'tagName', event.target.value)} /></td>
                                    {metadataColumns.map((column) => (
                                        <td key={column.id}>
                                            <input value={row.metadata?.[column.id] || ''} onChange={(event) => updateNodeMetadata(row.id, column.id, event.target.value)} />
                                        </td>
                                    ))}
                                    <td>
                                        <button type="button" className="opcua-icon-btn" onClick={() => removeNodeRow(row.id)} aria-label="Delete node row">
                                            <Trash2 size={16} />
                                        </button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
                <button type="button" className="opcua-add-metadata-btn" onClick={addMetadataColumn}>
                    <Plus size={16} />
                    <span>Add Metadata</span>
                </button>
            </div>
            <button type="button" className="opcua-outline-btn opcua-add-row-btn" onClick={addNodeRow}>
                <Plus size={16} />
                Add Row
            </button>
        </div>
    );

    const renderProcessorConditions = () => (
        <div className="opcua-processor-panel">
            <div className="opcua-condition-list">
                {conditionRows.map((row, index) => (
                    <div className="opcua-condition-card" key={row.id}>
                        <div className="opcua-condition-header">
                            <strong>Condition {index + 1}</strong>
                            <span>{getConditionExpression(row)}</span>
                            <button type="button" className="opcua-icon-btn" onClick={() => removeConditionRow(row.id)} aria-label="Delete condition">
                                <Trash2 size={16} />
                            </button>
                        </div>
                        <label className="opcua-action-label">If Condition</label>
                        <div className="opcua-clause-list">
                            {row.clauses.map((clause, clauseIndex) => (
                                <React.Fragment key={clause.id}>
                                    {clauseIndex === 1 && (
                                        <button type="button" className="opcua-clause-joiner" onClick={() => toggleConditionJoiner(row.id)}>
                                            {row.joiner}
                                        </button>
                                    )}
                                    <div className="opcua-condition-rule">
                                        <span aria-hidden="true" className="opcua-clause-grip">::</span>
                                        <select value={clause.field} onChange={(event) => updateConditionClause(row.id, clause.id, 'field', event.target.value)}>
                                            {conditionFieldOptions.map((option) => <option key={option.label}>{option.label}</option>)}
                                        </select>
                                        <select value={clause.operator} onChange={(event) => updateConditionClause(row.id, clause.id, 'operator', event.target.value)}>
                                            {conditionOperatorOptions.map((option) => <option key={option}>{option}</option>)}
                                        </select>
                                        <input value={clause.value} onChange={(event) => updateConditionClause(row.id, clause.id, 'value', event.target.value)} placeholder="Enter value..." />
                                        <button
                                            type="button"
                                            className="opcua-icon-btn"
                                            onClick={() => removeConditionClause(row.id, clause.id)}
                                            disabled={row.clauses.length === 1}
                                            aria-label="Delete clause"
                                        >
                                            <Trash2 size={16} />
                                        </button>
                                    </div>
                                </React.Fragment>
                            ))}
                        </div>
                        {row.clauses.length < 2 && (
                            <button type="button" className="opcua-add-clause-btn" onClick={() => addConditionClause(row.id)}>
                                <Plus size={16} />
                                Add Clause
                            </button>
                        )}
                        <label className="opcua-action-label">Then Action</label>
                        <div className="opcua-condition-action-editor">
                            <pre className="opcua-condition-action-lines">{getLineNumbers(row.action)}</pre>
                            <textarea
                                className="opcua-action-textarea"
                                value={row.action}
                                onChange={(event) => updateConditionAction(row.id, event.target.value)}
                                onKeyDown={(event) => handleYamlKeyDown(event, row.action, (value) => updateConditionAction(row.id, value))}
                                rows={6}
                                spellCheck={false}
                            />
                        </div>
                    </div>
                ))}
            </div>
            <button type="button" className="opcua-outline-btn" onClick={addConditionRow}>
                <Plus size={16} />
                Add Condition
            </button>
        </div>
    );

    const renderProcessorDownsampler = () => (
        <div className="opcua-downsampler-summary" aria-label="Downsampler default settings">
            <p>
                Automatically filters duplicate values to reduce data volume. <button type="button">View downsampler documentation for more information.</button>
            </p>
            <h4>Default settings</h4>
            <ul>
                <li>Threshold: {downsamplerSettings.threshold}</li>
                <li>Heartbeat interval: {downsamplerSettings.heartbeatInterval} {downsamplerSettings.heartbeatUnit}</li>
            </ul>
        </div>
    );

    const renderProcessorVisual = () => {
        if (processorSection === 'nodeIds') {
            return renderProcessorNodeIds();
        }

        if (processorSection === 'conditions') {
            return renderProcessorConditions();
        }

        if (processorSection === 'downsampler') {
            return renderProcessorDownsampler();
        }

        return renderProcessorDefaults();
    };

    const renderOpcuaProcessor = () => (
        <div className="opcua-readflow-card opcua-processing-card">
            <div className="opcua-readflow-header">
                <div>
                    <h2>Processing (Tag Processor)</h2>
                    <p>Transform time-series data using the tag processor. Best for industrial equipment data with tag names, timestamps, and values.</p>
                </div>

                <div className={`opcua-view-toggle ${isDownsamplerActive ? 'disabled' : ''}`} aria-label="Processing view mode">
                    <button
                        type="button"
                        className={processorView === 'Visual' ? 'active' : ''}
                        onClick={() => setProcessorView('Visual')}
                        disabled={isDownsamplerActive}
                    >
                        Visual
                    </button>
                    <button
                        type="button"
                        className={processorView === 'Code' ? 'active' : ''}
                        onClick={() => setProcessorView('Code')}
                        disabled={isDownsamplerActive}
                    >
                        Code
                    </button>
                </div>
            </div>

            <div className="opcua-visual-shell">
                <div className="opcua-section-nav opcua-processing-nav" aria-label="OPC UA processing sections">
                    {processorSections.map((section) => (
                        <button
                            key={section.id}
                            type="button"
                            className={processorSection === section.id ? 'active' : ''}
                            onClick={() => {
                                setProcessorSection(section.id);
                                if (section.id === 'downsampler') {
                                    setProcessorView('Visual');
                                }
                            }}
                        >
                            <SlidersHorizontal size={18} />
                            <span>
                                <strong>{section.title}</strong>
                                <small>{section.subtitle}</small>
                            </span>
                            {section.count && <em>{section.count}</em>}
                        </button>
                    ))}
                </div>

                <div className="opcua-selected-heading">
                    <h3>{activeProcessorSection.title === 'Default' ? 'Global Defaults' : activeProcessorSection.title}</h3>
                </div>

                <div className="opcua-section-content">
                    {processorView === 'Visual' || isDownsamplerActive ? renderProcessorVisual() : (
                        <div className="opcua-code-editor" onClick={() => setIsEditingProcessorCode(true)}>
                            {!isEditingProcessorCode && <span className="opcua-code-edit-label">Click to edit</span>}
                            <pre className="opcua-code-lines">{getLineNumbers(getProcessorCodeValue())}</pre>
                            <textarea
                                className="opcua-code-textarea"
                                value={getProcessorCodeValue()}
                                onChange={(event) => setProcessorCodeValue(event.target.value)}
                                onFocus={() => setIsEditingProcessorCode(true)}
                                onBlur={() => setIsEditingProcessorCode(false)}
                                onKeyDown={(event) => handleYamlKeyDown(event, getProcessorCodeValue(), setProcessorCodeValue)}
                                spellCheck={false}
                            />
                            <button
                                type="button"
                                className="opcua-code-expand-btn"
                                onClick={(event) => {
                                    event.stopPropagation();
                                    setFullscreenProcessorCode(true);
                                }}
                                aria-label="Expand OPC UA processor code"
                            >
                                <Maximize2 size={18} />
                            </button>
                        </div>
                    )}
                </div>
            </div>

            {fullscreenProcessorCode && (
                <div className="opcua-code-modal-overlay" onClick={() => setFullscreenProcessorCode(false)}>
                    <div className="opcua-code-modal" onClick={(event) => event.stopPropagation()}>
                        <div className="opcua-code-modal-header">
                            <h3>{activeProcessorSection.title === 'Default' ? 'Global Defaults' : activeProcessorSection.title} - Code</h3>
                            <button
                                type="button"
                                className="opcua-code-modal-close"
                                onClick={() => setFullscreenProcessorCode(false)}
                                aria-label="Close OPC UA processor code editor"
                            >
                                <X size={26} />
                            </button>
                        </div>
                        <div className="opcua-code-modal-editor">
                            <pre className="opcua-code-modal-lines">{getLineNumbers(getProcessorCodeValue())}</pre>
                            <textarea
                                className="opcua-code-modal-textarea"
                                value={getProcessorCodeValue()}
                                onChange={(event) => setProcessorCodeValue(event.target.value)}
                                onKeyDown={(event) => handleYamlKeyDown(event, getProcessorCodeValue(), setProcessorCodeValue)}
                                spellCheck={false}
                            />
                        </div>
                    </div>
                </div>
            )}
        </div>
    );

    return (
        <div className="opcua-readflow-scope">
        <div className="opcua-readflow-card">
            <div className="opcua-readflow-header">
                <div>
                    <h2>Input (OPC UA)</h2>
                    <p>
                        Configure OPC UA tags graphically or switch to YAML for advanced control.
                    </p>
                </div>

                <div className="opcua-view-toggle" aria-label="Input view mode">
                    <button
                        type="button"
                        className={viewMode === 'Visual' ? 'active' : ''}
                        onClick={() => setViewMode('Visual')}
                    >
                        Visual
                    </button>
                    <button
                        type="button"
                        className={viewMode === 'Code' ? 'active' : ''}
                        onClick={() => setViewMode('Code')}
                    >
                        Code
                    </button>
                </div>
            </div>

            {viewMode === 'Visual' ? (
                <div className="opcua-visual-shell">
                    <div className="opcua-section-nav" aria-label="OPC UA input sections">
                        <button
                            type="button"
                            className={activeSection === 'standard' ? 'active' : ''}
                            onClick={() => setActiveSection('standard')}
                        >
                            <SlidersHorizontal size={18} />
                            <span>
                                <strong>Standard & Advanced</strong>
                                <small>Connection and security</small>
                            </span>
                        </button>

                        <button
                            type="button"
                            className={activeSection === 'additional' ? 'active' : ''}
                            onClick={() => setActiveSection('additional')}
                        >
                            <Settings2 size={18} />
                            <span>
                                <strong>Additional Settings</strong>
                                <small>Custom YAML fields</small>
                            </span>
                            {additionalCount > 0 && <em>{additionalCount}</em>}
                        </button>
                    </div>

                    <div className="opcua-selected-heading">
                        <h3>{activeSection === 'standard' ? 'Standard & Advanced Settings' : 'Additional Settings'}</h3>
                    </div>

                    <div className="opcua-section-content">
                        {activeSection === 'standard' ? renderStandardAdvancedSettings() : renderAdditionalSettings()}
                    </div>
                </div>
            ) : (
                <div
                    className="opcua-code-editor"
                    onClick={() => setIsEditingCode(true)}
                >
                    {!isEditingCode && <span className="opcua-code-edit-label">Click to edit</span>}
                    <pre className="opcua-code-lines">{getLineNumbers(inputYaml)}</pre>
                    <textarea
                        className="opcua-code-textarea"
                        value={inputYaml}
                        onChange={(event) => handleCodeChange(event.target.value)}
                        onFocus={() => setIsEditingCode(true)}
                        onBlur={() => setIsEditingCode(false)}
                        onKeyDown={(event) => handleYamlKeyDown(event, inputYaml, setInputYaml)}
                        spellCheck={false}
                    />
                    <button
                        type="button"
                        className="opcua-code-expand-btn"
                        onClick={(event) => {
                            event.stopPropagation();
                            openFullscreenEditor('input');
                        }}
                        aria-label="Expand OPC UA input YAML"
                    >
                        <Maximize2 size={18} />
                    </button>
                </div>
            )}
        </div>
        {renderOpcuaProcessor()}
        </div>
    );
};

export { defaultOpcuaYaml, buildOpcuaYaml, parseOpcuaYaml };
export default OpcuaReadflow;
