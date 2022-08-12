import { Story, Meta } from '@storybook/angular/types-6-0';
import { ToggleComponent } from './toggle.component';

export default {
	title: 'Example/Toggle',
	component: ToggleComponent,
	argTypes: {
		isChecked: {
			control: { type: 'boolean' }
		},
		label: {
			control: { type: 'string' }
		}
	}
} as Meta;


