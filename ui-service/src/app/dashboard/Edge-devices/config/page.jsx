import React, { Suspense } from 'react';
import Getconfig from '../../../../components/Getconfig.jsx';

export default function GetConfigPage() {
    return (
        <Suspense fallback={null}>
            <Getconfig />
        </Suspense>
    );
}
