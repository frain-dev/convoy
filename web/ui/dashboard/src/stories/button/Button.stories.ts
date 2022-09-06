import { Story, Meta } from '@storybook/angular/types-6-0';
import { ButtonComponent } from '../../app/components/button/button.component';

export default {
	title: 'Example/Button',
	component: ButtonComponent,
	argTypes: {
		color: {
			options: ['primary', 'success', 'warning', 'danger', 'grey'],
			control: { type: 'select' },
			defaultValue: 'primary'
		},
		size: {
			options: ['lg', 'md', 'sm', 'xs'],
			control: { type: 'select' },
			defaultValue: 'md'
		},
		buttonText: {
			control: { type: 'text' }
		},
		type: {
			options: ['default', 'outline', 'clear', 'text', 'link', 'icon'],
			control: { type: 'select' },
			defaultValue: 'default'
		},
		texture: {
			options: ['deep', 'light'],
			control: { type: 'select' },
			defaultValue: 'deep'
		}
	},
	parameters: {
		actions: {
			handles: ['click']
		}
	}
} as Meta;

const Template: Story<ButtonComponent> = (args: ButtonComponent) => ({
	props: args
});

export const Base = Template.bind({});
Base.args = {
	buttonText: 'Button',
	color: 'primary',
	type: 'default',
	size: 'md',
	texture: 'deep'
};

export const Outline = Template.bind({});
Outline.args = {
	buttonText: 'Button',
	color: 'danger',
	type: 'outline',
	size: 'md',
	texture: 'deep'
};

export const Clear = Template.bind({});
Clear.args = {
	buttonText: 'Button',
	color: 'success',
	type: 'clear',
	size: 'md',
	texture: 'deep'
};

export const Link = Template.bind({});
Link.args = {
	buttonText: 'Button',
	color: 'primary',
	type: 'link',
	size: 'md',
	texture: 'deep'
};

export const Text = Template.bind({});
Text.args = {
	buttonText: 'Button',
	color: 'primary',
	type: 'text',
	size: 'md',
	texture: 'deep'
};

export const Icon = Template.bind({});
Icon.args = {
	buttonText: 'Button',
	color: 'primary',
	type: 'icon',
	size: 'md',
	texture: 'deep'
};
