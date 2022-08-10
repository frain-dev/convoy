import { Story, Meta } from '@storybook/angular/types-6-0';
import { InputComponent } from './input.component';

export default {
	title: 'Example/Input',
	component: InputComponent,
	argTypes: {
		label: {
			control: { type: 'text' }
		},
		name: {
			control: { type: 'text' }
		},
		type: {
			options: ['text', 'password', 'number', 'url', 'email'],
			control: { type: 'text' },
			defaultValue: 'text'
		},
		formControlName: {
			control: { type: 'text' }
		},
		autocomplete: {
			control: { type: 'text' }
		},
		errorMessage: {
			control: { type: 'text' }
		},
		placeholder: {
			control: { type: 'text' }
		},
		required: {
			control: { type: 'boolean' }
		},
		readonly: {
			control: { type: 'boolean' }
		}
	},
	parameters: {
		actions: {
			handles: ['click']
		}
	}
} as Meta;

const Template: Story<InputComponent> = (args: InputComponent) => ({
	props: args
});

export const Default = Template.bind({});
Default.args = {
	label: 'Input Label',
	name: 'inputName',
	type: 'text',
	placeholder: 'input placeholder',
	errorMessage: 'input error message'
};
