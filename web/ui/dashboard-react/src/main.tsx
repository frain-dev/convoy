import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from '@tanstack/react-router';
import { router } from './router';

import './index.css';

let root = document.getElementById('root');
if (!root) {
	const div = document.createElement('div');
	div.id = 'root';
	document.body.prepend(div);
	root = document.getElementById('root') as HTMLElement;
}

createRoot(root).render(
	<StrictMode>
		<RouterProvider router={router} />
	</StrictMode>,
);
