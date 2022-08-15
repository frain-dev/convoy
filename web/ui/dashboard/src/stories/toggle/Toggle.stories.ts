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
const Template: Story<ToggleComponent> = (args: ToggleComponent) => ({
	props: args
});

export const Base = Template.bind({});
Base.args = {
	isChecked: false
};
