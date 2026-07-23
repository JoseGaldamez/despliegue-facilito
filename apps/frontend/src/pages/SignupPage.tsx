import { useState, type FormEvent } from 'react';
import { Link, Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

export function SignupPage() {
  const { user, loading, error, signUpWithEmail, clearError } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-brand-green" />
      </div>
    );
  }

  if (user) {
    return <Navigate to="/" replace />;
  }

  const handleSignUp = async (e: FormEvent) => {
    e.preventDefault();
    setLocalError(null);

    if (password !== confirmPassword) {
      setLocalError('Las contraseñas no coinciden.');
      return;
    }

    setSubmitting(true);
    await signUpWithEmail(email, password);
    setSubmitting(false);
  };

  const displayError = localError || error;

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-md space-y-8">
        <div className="text-center">
          <h1 className="text-3xl font-bold text-brand-green">Despliegue Facilito</h1>
          <p className="mt-2 text-gray-500">Crea tu cuenta</p>
        </div>

        {displayError && (
          <div className="rounded-lg bg-red-50 border border-red-200 p-4" role="alert">
            <div className="flex items-start">
              <p className="text-sm text-red-700 flex-1">{displayError}</p>
              <button
                onClick={() => {
                  setLocalError(null);
                  clearError();
                }}
                className="ml-2 text-red-500 hover:text-red-700"
                aria-label="Dismiss error"
              >
                ✕
              </button>
            </div>
          </div>
        )}

        <form onSubmit={handleSignUp} className="space-y-4">
          <div>
            <label htmlFor="email" className="block text-sm font-medium text-gray-700">
              Email
            </label>
            <input
              id="email"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-gray-200 bg-white px-3 py-2.5 text-gray-900 placeholder-gray-400 focus:border-brand-green focus:outline-none focus:ring-1 focus:ring-brand-green"
              placeholder="tu@email.com"
            />
          </div>
          <div>
            <label htmlFor="password" className="block text-sm font-medium text-gray-700">
              Contraseña
            </label>
            <input
              id="password"
              type="password"
              required
              minLength={6}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-gray-200 bg-white px-3 py-2.5 text-gray-900 placeholder-gray-400 focus:border-brand-green focus:outline-none focus:ring-1 focus:ring-brand-green"
              placeholder="••••••••"
            />
          </div>
          <div>
            <label htmlFor="confirm-password" className="block text-sm font-medium text-gray-700">
              Confirmar contraseña
            </label>
            <input
              id="confirm-password"
              type="password"
              required
              minLength={6}
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-gray-200 bg-white px-3 py-2.5 text-gray-900 placeholder-gray-400 focus:border-brand-green focus:outline-none focus:ring-1 focus:ring-brand-green"
              placeholder="••••••••"
            />
          </div>
          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-lg bg-brand-green px-4 py-3 text-sm font-semibold text-white hover:bg-brand-green-dark focus:outline-none focus:ring-2 focus:ring-brand-green focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {submitting ? 'Creando cuenta...' : 'Crear cuenta'}
          </button>
        </form>

        <p className="text-center text-sm text-gray-500">
          ¿Ya tienes cuenta?{' '}
          <Link to="/login" className="font-medium text-brand-green hover:text-brand-green-dark">
            Iniciar sesión
          </Link>
        </p>
      </div>
    </div>
  );
}
