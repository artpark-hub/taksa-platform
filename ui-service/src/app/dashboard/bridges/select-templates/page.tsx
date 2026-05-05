import { Suspense } from 'react';
import SelectTemplates from '../../../../components/SelectTemplates';

export default function SelectTemplatesPage() {
    return (
        <Suspense>
            <SelectTemplates />
        </Suspense>
    );
}