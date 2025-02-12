import convoyLogo from './assets/img/svg/convoy-logo-full-new.svg';

function App() {
	return (
		<>
			<div className="">
				<a href="/">
					<img src={convoyLogo} className="logo convoy" alt="Convoy logo" />
				</a>
				<h1 className="text-3xl font-bold underline">Convoy</h1>
			</div>
		</>
	);
}

export default App;
