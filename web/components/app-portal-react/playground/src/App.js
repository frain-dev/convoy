import { ConvoyApp } from 'convoy-app-react';
import 'convoy-app-react/dist/index.css';

function App() {
	return (
		<div className="App">
			<ConvoyApp
				token={'CO.WRpze8A0vAb19rbt.4W39y6csBPxsV90UPCMIX3vzpYBKU5R5GbNYjE3N6tpgdDKIrqHVDbADjQ1QuOJc'}
				apiURL={'http://localhost:5005'}
				appId={'291e98cb-4e93-408f-bb5b-d422ff13d12c'}
				groupId={'5c9c6db0-7606-4f9f-9965-5455980881a2'}
			/>
		</div>
	);
}

export default App;
