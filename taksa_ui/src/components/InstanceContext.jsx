import React, { createContext, useState, useContext } from 'react';

// Create the context
const InstanceContext = createContext();

// Create the provider component
export const InstanceProvider = ({ children }) => {
    // --- PERSISTENT STATE (Resets on Reload) ---
    const [showDiscovery, setShowDiscovery] = useState(false); // <--- MOVED HERE

    const [instances, setInstances] = useState([
        {
            id: 1,
            serial: '1',
            status: 'inactive',
            name: 'Mitsubishi PLC',
            type: 'PLC',
            version: 'v0.44.6',
            updateAvailable: true,
            dataFlows: 3,
            topics: 0,
            latency: 0,
            throughput: 0.00
        },
    ]);

    // Function to update status globally
    const updateInstanceStatus = (id, newStatus) => {
        setInstances(prev => prev.map(inst =>
            inst.id === id ? { ...inst, status: newStatus } : inst
        ));
    };

    return (
        <InstanceContext.Provider value={{
            instances,
            updateInstanceStatus,
            showDiscovery,      // <--- Expose state
            setShowDiscovery    // <--- Expose setter
        }}>
            {children}
        </InstanceContext.Provider>
    );
};

// Custom hook to use this context easily
export const useInstanceContext = () => useContext(InstanceContext);