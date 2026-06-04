import { Suspense } from 'react';
import TopicBrowser from '../../../components/TopicBrowser';

export default function DataFlowsPage() {
    return (
        <Suspense fallback={null}>
            <TopicBrowser />
        </Suspense>
    );
}
