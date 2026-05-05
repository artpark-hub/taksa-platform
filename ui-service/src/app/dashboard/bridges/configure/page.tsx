import { Suspense } from 'react';
import BridgeConfiguration from '../../../../components/bridge-configuration/BridgeConfiguration';

export default function BridgeConfigurePage() {
    return (
        <Suspense>
            <BridgeConfiguration />
        </Suspense>
    );
}
