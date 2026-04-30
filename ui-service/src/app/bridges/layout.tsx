import React from 'react';
import DashboardLayoutClient from '../../components/Dashboard/DashboardLayoutClient';

export const metadata = {
    title: "Taksa Bridges",
};

export default function BridgesLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return (
        <DashboardLayoutClient>
            {children}
        </DashboardLayoutClient>
    );
}
