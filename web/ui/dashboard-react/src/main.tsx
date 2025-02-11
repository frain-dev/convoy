import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import App from './App.tsx';

let root = document.getElementById('root');
if (!root) {
	const div = document.createElement('div');
	div.id = 'root';
	document.body.prepend(div);
	root = document.getElementById('root') as HTMLElement;
}

createRoot(root).render(
	<StrictMode>
		<App />
	</StrictMode>,
);
