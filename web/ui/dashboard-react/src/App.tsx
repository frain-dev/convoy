import convoyLogo from './assets/convoy-logo-full-new.svg';
import './App.css';

function App() {
	return (
		<>
			<div>
				<a href="/">
					<img src={convoyLogo} className="logo convoy" alt="Convoy logo" />
				</a>
			</div>
		</>
	);
}

export default App;
