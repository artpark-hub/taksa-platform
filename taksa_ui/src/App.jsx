import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './components/Login';
import Register from './components/Register';
import DashboardLayout from './components/DashboardLayout';
import DataFlow from './components/DataFlow';
import Instances from './components/Instances';
import Visualise from './components/Visualise';
import InstanceDetails from './components/InstanceDetails';
import { InstanceProvider } from './components/InstanceContext';
import './App.css';

function App() {
  return (
    <InstanceProvider>
      <BrowserRouter>
        <Routes>
          {/* Public Routes */}
          <Route path="/" element={<Login />} />
          <Route path="/Login" element={<Login />} />
          <Route path="/Register" element={<Register />} />

          <Route element={<DashboardLayout />}>
            <Route path="/Dashboard" element={<Navigate to="/data-flow" replace />} />
            <Route path="/data-flow" element={<DataFlow />} />
            <Route path="/instances" element={<Instances />} />
            <Route path="/visualise" element={<Visualise />} />
            <Route path="/InstanceDetails" element={<InstanceDetails />} />
          </Route>

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </InstanceProvider>
  );
}

export default App;