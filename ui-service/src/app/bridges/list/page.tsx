import { Suspense } from 'react';
import Bridges from '../../../components/Bridges';

export default function BridgeListPage() {
    return (
        <Suspense>
            <Bridges />
        </Suspense>
    );
}
