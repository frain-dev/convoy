import React from 'react';

import { ConvoyApp } from 'convoy-app-react';
import 'convoy-app-react/dist/index.css';

const App = () => {
	return (
		<div>
			<ConvoyApp
				token={'CO.d3Y6OIbYnIZvTifX.HaLXVbRDzQrFODNUzD96GQRURI7i6PQXquYWshrlGXcNj8hjgJweSaoOpiNHr4V7'}
				apiURL={'http://localhost:5005'}
			/>
		</div>
	);
};

export default App;
