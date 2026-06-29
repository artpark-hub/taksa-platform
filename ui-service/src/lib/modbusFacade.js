const normalizeProtocolKey = (protocol) => String(protocol || '')
    .trim()
    .toLowerCase()
    .replace(/[\s_-]/g, '');

const ensureTrailingNewline = (value, fallback = '') => {
    const normalized = String(value ?? fallback);
    return normalized.endsWith('\n') ? normalized : `${normalized}\n`;
};

const defaultBufferYaml = 'buffer:\n  none: {}\n';

const addTemplateVariables = (target, source) => {
    if (!source) {
        return;
    }

    if (Array.isArray(source)) {
        source.forEach((item) => {
            const label = String(item?.label ?? item?.key ?? '').trim();
            if (!label) {
                return;
            }
            target[label] = String(item?.value ?? '');
        });
        return;
    }

    if (typeof source === 'object') {
        Object.entries(source).forEach(([key, value]) => {
            const label = String(key || '').trim();
            if (!label) {
                return;
            }
            target[label] = String(value ?? '');
        });
    }
};

const templateVariablesToArray = (variables) => {
    if (Array.isArray(variables)) {
        return variables.map((item) => ({
            label: String(item?.label ?? item?.key ?? '').trim(),
            value: String(item?.value ?? '')
        })).filter((item) => item.label);
    }

    if (!variables || typeof variables !== 'object') {
        return [];
    }

    return Object.entries(variables)
        .map(([label, value]) => ({ label, value: String(value ?? '') }))
        .filter((item) => item.label);
};

const normalizeLocationLevels = (location) => {
    if (!location || typeof location !== 'object') {
        return {};
    }

    const levels = location.levels && typeof location.levels === 'object'
        ? location.levels
        : location;

    return Object.entries(levels).reduce((acc, [key, value]) => {
        if (/^\d+$/.test(String(key))) {
            acc[String(key)] = String(value ?? '');
        }
        return acc;
    }, {});
};

const normalizeFacadeResult = (result) => result?.result || result?.data?.result || result?.data || result || {};

export const isModbusProtocol = (protocol) => {
    const normalized = normalizeProtocolKey(protocol);
    return normalized === 'modbus' || normalized === 'modbustcp' || normalized.startsWith('modbus');
};

export const buildModbusFacadeRequest = ({
    bridgeConfig,
    deviceId,
    uuid,
    location,
    port,
    includeApplyReadConfig = false
}) => {
    const parsedPort = Number.isFinite(port)
        ? port
        : Number.parseInt(String(bridgeConfig?.port || ''), 10);
    const ip = String(bridgeConfig?.ipAddress || '').trim();
    const templateVariables = {
        IP: ip,
        PORT: String(Number.isFinite(parsedPort) ? parsedPort : '')
    };

    addTemplateVariables(templateVariables, bridgeConfig?.templateVariables);
    addTemplateVariables(templateVariables, bridgeConfig?.readTemplateVariables);

    const body = {
        deviceId,
        name: String(bridgeConfig?.name || '').trim(),
        state: bridgeConfig?.state || 'active',
        connection: {
            ip,
            port: Number.isFinite(parsedPort) ? parsedPort : 0
        },
        location: {
            levels: normalizeLocationLevels(location)
        },
        input: {
            mode: 'RAW',
            rawYaml: ensureTrailingNewline(bridgeConfig?.readInputYaml)
        },
        readFlow: {
            dataType: 'TIME_SERIES',
            processorMode: 'RAW',
            rawProcessorYaml: ensureTrailingNewline(bridgeConfig?.readProcessorYaml),
            bufferMode: 'RAW',
            rawBufferYaml: ensureTrailingNewline(bridgeConfig?.readRawYamlInject, defaultBufferYaml)
        },
        templateVariables
    };

    if (uuid) {
        body.uuid = uuid;
    }

    if (includeApplyReadConfig) {
        body.applyReadConfig = true;
    }

    return body;
};

export const modbusFacadeResultToBridgeConfig = ({
    result,
    selectedDeviceName = '',
    fallbackBridgeId = ''
}) => {
    const facade = normalizeFacadeResult(result);
    const levels = normalizeLocationLevels(facade?.location);
    const normalizedLevels = Object.entries(levels)
        .filter(([key]) => /^\d+$/.test(String(key)) && String(key) !== '0')
        .sort((a, b) => Number(a[0]) - Number(b[0]))
        .map(([key, value], index) => ({
            key: `level${key}`,
            index: Number(key),
            label: `Level ${key}`,
            value: String(value ?? ''),
            isUserAdded: index > 0
        }));

    const templateVariables = templateVariablesToArray(facade?.templateVariables);
    const readTemplateVariables = templateVariables.filter((item) => {
        const label = item.label.toUpperCase();
        return label !== 'IP' && label !== 'PORT';
    });
    const readFlow = facade?.readFlow || {};
    const input = facade?.input || {};

    return {
        name: facade?.name || '',
        instance: selectedDeviceName,
        level0: String(levels?.['0'] || ''),
        levels: normalizedLevels,
        protocol: 'Modbus',
        dataType: 'Time Series',
        ipAddress: facade?.connection?.ip || '',
        port: String(facade?.connection?.port || ''),
        readInputType: 'modbus',
        readInputYaml: facade?.rawInputYaml || input?.rawYaml || '',
        readProcessorType: 'tag_processor',
        readProcessorYaml: facade?.rawProcessorYaml || readFlow?.rawProcessorYaml || '',
        readRawYamlInject: facade?.rawBufferYaml || readFlow?.rawBufferYaml || readFlow?.yamlInject?.rawYaml || defaultBufferYaml,
        metaProtocol: 'modbus_tcp',
        bridgeUuid: facade?.uuid || fallbackBridgeId,
        templateVariables,
        readTemplateVariables
    };
};
