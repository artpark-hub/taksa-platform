import {
    buildProtocolFacadeRequest,
    facadeResultToBridgeConfig,
    normalizeProtocolKey
} from './protocolFacadeShared';

export const isModbusProtocol = (protocol) => {
    const normalized = normalizeProtocolKey(protocol);
    return normalized === 'modbus' || normalized === 'modbustcp' || normalized.startsWith('modbus');
};

export const buildModbusFacadeRequest = (options) => buildProtocolFacadeRequest(options);

export const modbusFacadeResultToBridgeConfig = (options) => facadeResultToBridgeConfig({
    ...options,
    protocol: 'Modbus',
    readInputType: 'modbus',
    metaProtocol: 'modbus_tcp'
});
