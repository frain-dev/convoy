import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from '@tanstack/react-router';

import './index.css';
import { router } from './lib/router';
import { useAuth } from './hooks/use-auth';

let root = document.getElementById('root');
if (!root) {
	const div = document.createElement('div');
	div.id = 'root';
	document.body.prepend(div);
	root = document.getElementById('root') as HTMLElement;
}

export function App() {
	const auth = useAuth();
	return <RouterProvider router={router} context={{ auth }} />;
}

createRoot(root).render(
	<StrictMode>
		<App />
	</StrictMode>,
);
