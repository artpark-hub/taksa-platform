import {
    buildProtocolFacadeRequest,
    facadeResultToBridgeConfig,
    normalizeProtocolKey
} from './protocolFacadeShared';

export const isOpcUaProtocol = (protocol) => {
    const normalized = normalizeProtocolKey(protocol);
    return normalized === 'opcua' || normalized === 'benthosopcua';
};

export const buildOpcUaFacadeRequest = (options) => buildProtocolFacadeRequest(options);

export const opcUaFacadeResultToBridgeConfig = (options) => facadeResultToBridgeConfig({
    ...options,
    protocol: 'OPCUA',
    readInputType: 'opcua',
    metaProtocol: 'opcua'
});
