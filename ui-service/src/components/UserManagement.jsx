'use client';

import React, { useEffect, useState } from 'react';
import { Search, Plus, Filter, X, Eye, EyeOff, CheckCircle2, AlertCircle, AlertTriangle, Trash2, Edit } from 'lucide-react';
import { useRouter } from 'next/navigation';
import './UserManagement.css';

const normalizeRole = (role) => (role || '').trim().toLowerCase();

const formatRoleLabel = (role) => {
    const sanitizedRole = (role || '').trim();

    if (!sanitizedRole) {
        return 'Unknown';
    }

    if (normalizeRole(sanitizedRole) === 'master') {
        return 'Super-admin';
    }

    return sanitizedRole
        .split(/[_\s-]+/)
        .filter(Boolean)
        .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1).toLowerCase())
        .join(' ');
};




const UserManagement = () => {
    const router = useRouter();
    const [currentUser, setCurrentUser] = useState(null);
    const [users, setUsers] = useState([]);
    const [isLoading, setIsLoading] = useState(true);

    const [searchQuery, setSearchQuery] = useState('');
    const [roleFilter, setRoleFilter] = useState('All');

    const [isAddModalOpen, setIsAddModalOpen] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [successMessage, setSuccessMessage] = useState('');
    const [apiError, setApiError] = useState('');
    const [showPassword, setShowPassword] = useState(false);
    const [showConfirmPassword, setShowConfirmPassword] = useState(false);

    const [deleteModalConfig, setDeleteModalConfig] = useState({
        isOpen: false,
        user: null,
        isDeleting: false
    });

    const [formData, setFormData] = useState({
        firstName: '',
        lastName: '',
        email: '',
        role: '',
        password: '',
        confirmPassword: ''
    });
    const [formErrors, setFormErrors] = useState({});

    const availableRoleFilters = React.useMemo(() => {
        const uniqueRoles = Array.from(
            new Set(users.map((user) => normalizeRole(user.role)).filter(Boolean))
        ).sort();

        return ['All', ...uniqueRoles];
    }, [users]);

    const parseResponseSafely = async (response, fallbackMessage) => {
        const raw = await response.text();
        let data = {};

        try {
            data = raw ? JSON.parse(raw) : {};
        } catch (err) {
            if (!response.ok) {
                throw new Error(raw || fallbackMessage);
            }
            data = {};
        }

        if (!response.ok) {
            throw new Error(
                data?.message ||
                data?.error?.message ||
                data?.error ||
                fallbackMessage
            );
        }

        return data;
    };

    const fetchCurrentUser = async () => {
        try {
            const response = await fetch('/sessions/whoami', {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            if (!response.ok) {
                throw new Error('Failed to fetch current user');
            }

            const data = await response.json();
            const identity = data?.identity;
            const traits = identity?.traits || {};

            const userData = {
                identityId: identity?.id || '',
                email: traits?.email || '',
                firstName: traits?.name?.first || '',
                lastName: traits?.name?.last || '',
                role: traits?.role || '',
                organizationName: traits?.organization_name || ''
            };

            setCurrentUser(userData);
            // Keep internal state in camelCase, but store to localStorage in snake_case
            const storageUserData = {
                identity_id: userData.identityId,
                email: userData.email,
                first_name: userData.firstName,
                last_name: userData.lastName,
                role: userData.role,
                organization_name: userData.organizationName,
                // camelCase keys
                identityId: userData.identityId,
                firstName: userData.firstName,
                lastName: userData.lastName,
                organizationName: userData.organizationName
            };

            localStorage.setItem('taksa_user', JSON.stringify(storageUserData));
        } catch (error) {
            console.error("Error loading user data", error);
        }
    };

    const fetchUsers = async () => {
        setIsLoading(true);
        setApiError('');

        try {
            const response = await fetch(`/api/v1/um/list_all_users`, {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            const data = await parseResponseSafely(response, 'Failed to fetch users');

            const apiUsers = Array.isArray(data?.users)
                ? data.users
                : Array.isArray(data?.data?.users)
                    ? data.data.users
                    : [];

            const normalizedUsers = apiUsers.map((user) => ({
                identityId: user.identity_id || user.identityId || '',
                email: user.email || '',
                firstName: user.first_name || user.firstName || '',
                lastName: user.last_name || user.lastName || '',
                organizationName: user.organization_name || user.organizationName || '',
                role: user.role || 'sub'
            }));

            setUsers(normalizedUsers);
        } catch (error) {
            console.error("Error fetching users:", error);
            setUsers([]);
            setApiError(error.message || 'Failed to fetch users');
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        fetchCurrentUser();
        fetchUsers();
    }, []);

    const initiateDelete = (id) => {
        const userToDelete = users.find(u => u.identityId === id);
        if (userToDelete) {
            setDeleteModalConfig({ isOpen: true, user: userToDelete, isDeleting: false });
        }
    };

    const cancelDelete = () => {
        setDeleteModalConfig({ isOpen: false, user: null, isDeleting: false });
        setApiError('');
    };

    const confirmDelete = async () => {
        const { user } = deleteModalConfig;
        if (!user) return;

        const normalizedRole = normalizeRole(user.role);

        setDeleteModalConfig(prev => ({ ...prev, isDeleting: true }));
        setApiError('');

        try {
            const endpoint = normalizedRole === 'master'
                ? `/api/v1/um/delete_master_user/${user.identityId}`
                : `/api/v1/um/delete_sub_user/${user.identityId}`;

            const response = await fetch(`${endpoint}`, {
                method: 'DELETE',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            await parseResponseSafely(response, 'Failed to delete user');

            setSuccessMessage(`${formatRoleLabel(user.role)} deleted successfully!`);
            setDeleteModalConfig({ isOpen: false, user: null, isDeleting: false });
            await fetchUsers();

            setTimeout(() => setSuccessMessage(''), 3000);

        } catch (error) {
            console.error("Delete Error:", error);
            setApiError(error.message || 'Failed to delete user');
            setDeleteModalConfig(prev => ({ ...prev, isDeleting: false }));
        }
    };

    const handleInputChange = (e) => {
        const { name, value } = e.target;
        setFormData(prev => {
            const updated = { ...prev, [name]: value };

            setFormErrors(prevErrors => {
                const newErrors = { ...prevErrors };
                if (newErrors[name]) delete newErrors[name];

                if (name === 'password') {
                    if (updated.confirmPassword && value !== updated.confirmPassword) {
                        newErrors.confirmPassword = "Passwords do not match";
                    } else {
                        delete newErrors.confirmPassword;
                    }
                }

                if (name === 'confirmPassword') {
                    if (value && value !== updated.password) {
                        newErrors.confirmPassword = "Passwords do not match";
                    } else {
                        delete newErrors.confirmPassword;
                    }
                }

                return newErrors;
            });

            return updated;
        });
    };

    const validateEmail = (email) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);

    const handleCreateUser = async (e) => {
        e.preventDefault();
        const errors = {};
        setApiError('');
        setSuccessMessage('');

        if (!formData.firstName.trim()) errors.firstName = "First name is required";
        if (!formData.lastName.trim()) errors.lastName = "Last name is required";
        if (!formData.email.trim() || !validateEmail(formData.email)) errors.email = "Valid email is required";
        if (!formData.role.trim()) errors.role = "Role is required";
        if (!formData.password) errors.password = "Password is required";
        if (formData.password && formData.password.length < 8) errors.password = "Password must be at least 8 characters";
        if (!formData.confirmPassword) errors.confirmPassword = "Confirm password is required";
        else if (formData.password !== formData.confirmPassword) errors.confirmPassword = "Passwords do not match";

        if (Object.keys(errors).length > 0) {
            setFormErrors(errors);
            return;
        }

        setIsSubmitting(true);

        try {
            const endpoint = '/api/v1/um/create_sub_user';

            const payload = {
                email: formData.email.trim(),
                password: formData.password,
                first_name: formData.firstName.trim(),
                last_name: formData.lastName.trim(),
                role: formData.role.trim()
            };

            const response = await fetch(endpoint, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify(payload)
            });

            await parseResponseSafely(
                response,
                'Failed to create user'
            );

            setSuccessMessage('User successfully created!');
            await fetchUsers();

            setTimeout(() => {
                closeModal();
            }, 1500);

        } catch (error) {
            console.error("Creation Error:", error);
            setApiError(error.message || 'Failed to create user');
        } finally {
            setIsSubmitting(false);
        }
    };

    const closeModal = () => {
        setIsAddModalOpen(false);
        setSuccessMessage('');
        setApiError('');
        setShowPassword(false);
        setShowConfirmPassword(false);
        setFormData({ firstName: '', lastName: '', email: '', role: '', password: '', confirmPassword: '' });
        setFormErrors({});
    };

    const handleEditClick = (user) => {
        const queryParams = new URLSearchParams({
            id: user.identityId,
            firstName: user.firstName || '',
            lastName: user.lastName || '',
            email: user.email || '',
            role: user.role || ''
        }).toString();

        router.push(`/dashboard/users/edit?${queryParams}`);
    };

    const filteredUsers = users.filter(user => {
        const normalizedRole = normalizeRole(user.role);
        const fullName = `${user.firstName} ${user.lastName}`.toLowerCase();
        const matchesSearch = fullName.includes(searchQuery.toLowerCase()) || user.email.toLowerCase().includes(searchQuery.toLowerCase());
        const matchesRole = roleFilter === 'All' || normalizedRole === roleFilter.toLowerCase();
        return matchesSearch && matchesRole;
    });

    const isMaster = (currentUser?.role || '').toLowerCase() === 'master';

    return (
        <div className="user-management-container">
            <div className="um-header-container">
                <div>
                    <h1 className="um-title">User Management</h1>
                    <p className="um-subtitle">Manage roles, permissions, and organization access for all your team members.</p>
                </div>
            </div>
            {successMessage && !isAddModalOpen && !deleteModalConfig.isOpen && (
                <div className="um-success-alert" style={{ marginBottom: '15px' }}>
                    <CheckCircle2 size={18} /> {successMessage}
                </div>
            )}
            {apiError && !isAddModalOpen && !deleteModalConfig.isOpen && (
                <div className="um-success-alert" style={{ backgroundColor: '#fef2f2', color: '#dc2626', borderColor: '#fecaca', marginBottom: '15px' }}>
                    <AlertCircle size={18} /> {apiError}
                </div>
            )}

            <div className="um-controls-bar">
                <div className="search-filter-group">
                    <div className="search-wrapper">
                        <Search size={18} className="search-icon" />
                        <input
                            type="text"
                            className="search-input"
                            placeholder="Search users by name or email..."
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                        />
                    </div>

                    <div className="filter-wrapper">
                        <Filter size={18} className="filter-icon" />
                        <select
                            className="filter-select"
                            value={roleFilter}
                            onChange={(e) => setRoleFilter(e.target.value)}
                        >
                            {availableRoleFilters.map((roleOption) => (
                                <option key={roleOption} value={roleOption}>
                                    {roleOption === 'All' ? 'All Roles' : formatRoleLabel(roleOption)}
                                </option>
                            ))}
                        </select>
                    </div>
                </div>

                {isMaster && (
                    <button
                        className="btn-black btn-add-user"
                        onClick={() => {
                            setApiError('');
                            setSuccessMessage('');
                            setIsAddModalOpen(true);
                        }}
                    >
                        <Plus size={18} /> Add User
                    </button>
                )}
            </div>

            <div className="um-table-wrapper">
                <table className="um-table">
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Email</th>
                            <th>Role</th>
                            <th>Organization</th>
                            <th className="action-col">Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        {isLoading ? (
                            <tr><td colSpan="5" className="um-empty-state">Loading users...</td></tr>
                        ) : filteredUsers.length > 0 ? (
                            filteredUsers.map((user) => {
                                const normalizedRole = normalizeRole(user.role);
                                const isSelf = currentUser?.email === user.email;

                                const canEdit = isMaster || isSelf;
                                const canDelete = isMaster;

                                return (
                                    <tr key={user.identityId}>
                                        <td><span className="user-name-text">{user.firstName} {user.lastName}</span></td>
                                        <td>{user.email}</td>
                                        <td>
                                            <span className={`role-badge ${normalizedRole === 'master' ? 'super-admin' : 'super-user'}`}>
                                                {formatRoleLabel(user.role)}
                                            </span>
                                        </td>
                                        <td>{user.organizationName}</td>
                                        <td className="action-col">
                                            <div className="action-cell-inline">
                                                <button
                                                    className="action-btn-inline edit-btn"
                                                    onClick={() => handleEditClick(user)}
                                                    disabled={!canEdit}
                                                    style={{ opacity: canEdit ? 1 : 0.3, cursor: canEdit ? 'pointer' : 'not-allowed' }}
                                                    title="Edit User"
                                                >
                                                    <Edit size={18} />
                                                </button>

                                                <button
                                                    className="action-btn-inline delete-btn"
                                                    onClick={() => initiateDelete(user.identityId)}
                                                    disabled={!canDelete}
                                                    style={{ opacity: canDelete ? 1 : 0.3, cursor: canDelete ? 'pointer' : 'not-allowed' }}
                                                    title="Delete User"
                                                >
                                                    <Trash2 size={18} />
                                                </button>
                                            </div>
                                        </td>
                                    </tr>
                                );
                            })
                        ) : (
                            <tr>
                                <td colSpan="5" className="um-empty-state">
                                    No users found matching your search or filter.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>

            {deleteModalConfig.isOpen && (
                <div className="um-modal-overlay">
                    <div className="um-delete-modal">
                        <div className="um-delete-icon-wrapper">
                            <div className="um-delete-icon-bg">
                                <AlertTriangle size={28} color="#dc2626" />
                            </div>
                        </div>
                        <h3 className="um-delete-title">Delete User?</h3>
                        <p className="um-delete-desc">
                            Are you sure you want to delete <strong>{deleteModalConfig.user?.firstName} {deleteModalConfig.user?.lastName}</strong>?
                            This action cannot be undone and will permanently remove them from the organization.
                        </p>

                        {apiError && (
                            <div className="um-error-text" style={{ textAlign: 'center', marginBottom: '15px', fontSize: '0.85rem' }}>
                                {apiError}
                            </div>
                        )}

                        <div className="um-delete-actions">
                            <button
                                className="um-btn-cancel"
                                onClick={cancelDelete}
                                disabled={deleteModalConfig.isDeleting}
                            >
                                Cancel
                            </button>
                            <button
                                className="um-btn-danger"
                                onClick={confirmDelete}
                                disabled={deleteModalConfig.isDeleting}
                            >
                                {deleteModalConfig.isDeleting ? 'Deleting...' : 'Yes, Delete'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {isAddModalOpen && (
                <div className="um-modal-overlay">
                    <div className="um-adduser-modal">
                        <div className="um-modal-header">
                            <h2>Add User</h2>
                            <button type="button" className="um-modal-close-btn" onClick={closeModal}><X size={20} /></button>
                        </div>

                        <div className="um-modal-body">
                            {successMessage && (
                                <div className="um-success-alert">
                                    <CheckCircle2 size={18} /> {successMessage}
                                </div>
                            )}

                            {apiError && (
                                <div className="um-success-alert" style={{ backgroundColor: '#fef2f2', color: '#dc2626', borderColor: '#fecaca' }}>
                                    <AlertCircle size={18} /> {apiError}
                                </div>
                            )}

                            <form onSubmit={handleCreateUser}>
                                <div className="um-form-row-split">
                                    <div className="um-input-group">
                                        <label>First name <span className="um-req">*</span></label>
                                        <input
                                            type="text"
                                            name="firstName"
                                            value={formData.firstName}
                                            onChange={handleInputChange}
                                            className={formErrors.firstName ? 'um-input-error' : ''}
                                        />
                                        {formErrors.firstName && <span className="um-error-text">{formErrors.firstName}</span>}
                                    </div>
                                    <div className="um-input-group">
                                        <label>Last name <span className="um-req">*</span></label>
                                        <input
                                            type="text"
                                            name="lastName"
                                            value={formData.lastName}
                                            onChange={handleInputChange}
                                            className={formErrors.lastName ? 'um-input-error' : ''}
                                        />
                                        {formErrors.lastName && <span className="um-error-text">{formErrors.lastName}</span>}
                                    </div>
                                </div>

                                <div className="um-input-group">
                                    <label>Email <span className="um-req">*</span></label>
                                    <input
                                        type="email"
                                        name="email"
                                        placeholder="user@yourcompany.com"
                                        value={formData.email}
                                        onChange={handleInputChange}
                                        className={formErrors.email ? 'um-input-error' : ''}
                                    />
                                    {formErrors.email && <span className="um-error-text">{formErrors.email}</span>}
                                </div>

                                <div className="um-input-group">
                                    <label>Role <span className="um-req">*</span></label>
                                    <input
                                        type="text"
                                        name="role"
                                        placeholder="e.g. viewer, editor, manager"
                                        value={formData.role}
                                        onChange={handleInputChange}
                                        className={formErrors.role ? 'um-input-error' : ''}
                                    />
                                    {formErrors.role && <span className="um-error-text">{formErrors.role}</span>}
                                </div>

                                <div className="um-input-group">
                                    <label>Password <span className="um-req">*</span></label>
                                    <div className="um-password-wrapper">
                                        <input
                                            type={showPassword ? "text" : "password"}
                                            name="password"
                                            value={formData.password}
                                            onChange={handleInputChange}
                                            className={formErrors.password ? 'um-input-error' : ''}
                                        />
                                        <button
                                            type="button"
                                            className="um-eye-btn"
                                            onClick={() => setShowPassword(!showPassword)}
                                        >

                                            {showPassword ? <Eye size={18} /> : <EyeOff size={18} />}
                                        </button>
                                    </div>
                                    {formErrors.password && <span className="um-error-text">{formErrors.password}</span>}
                                </div>

                                <div className="um-input-group">
                                    <label>Confirm Password <span className="um-req">*</span></label>
                                    <div className="um-password-wrapper">
                                        <input
                                            type={showConfirmPassword ? "text" : "password"}
                                            name="confirmPassword"
                                            value={formData.confirmPassword}
                                            onChange={handleInputChange}
                                            className={formErrors.confirmPassword ? 'um-input-error' : ''}
                                        />
                                        <button
                                            type="button"
                                            className="um-eye-btn"
                                            onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                                        >
                                            {showConfirmPassword ? <Eye size={18} /> : <EyeOff size={18} />}
                                        </button>
                                    </div>
                                    {formErrors.confirmPassword && <span className="um-error-text">{formErrors.confirmPassword}</span>}
                                </div>

                                <button
                                    type="submit"
                                    className="um-btn-submit-modal"
                                    disabled={isSubmitting || successMessage !== ''}
                                >
                                    {isSubmitting
                                        ? 'Creating user...'
                                        : 'Create'}
                                </button>
                            </form>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default UserManagement;