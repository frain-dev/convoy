import { Story, Meta } from '@storybook/angular/types-6-0';
import { componentWrapperDecorator } from '@storybook/angular';
import { DropdownComponent } from './dropdown.component';
import * as ButtonStories from '../button/Button.stories';

export default {
	title: 'Example/Dropdown',
	component: DropdownComponent,
    decorators:[
        componentWrapperDecorator((story) => `<div style="margin: 3em">${story}</div>`),
    ],
	argTypes: {
		position: {
			options: ['right', 'left'],
			control: { type: 'select' },
			defaultValue: 'right'
		},
		size: {
			options: ['sm', 'md', 'lg', 'xl'],
			control: { type: 'select' },
			defaultValue: 'md'
		},
		buttonColor: {
			options: ['primary', 'success', 'warning', 'danger', 'grey'],
			control: { type: 'select' },
			defaultValue: 'primary'
		},
		buttonSize: {
			options: ['xs', 'sm', 'md', 'lg'],
			control: { type: 'select' },
			defaultValue: 'md'
		},
		buttonType: {
			options: ['default', 'outline', 'clear', 'text', 'link', 'icon'],
			control: { type: 'select' },
			defaultValue: 'default'
		},
		buttonTexture: {
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

const Template: Story<DropdownComponent> = (args: DropdownComponent) => ({
	props: args
});

export const Default = Template.bind({});
Default.args = {
	position: 'right',
	size: 'md',
	buttonColor: ButtonStories.Base.args?.color,
	buttonSize: ButtonStories.Base.args?.size,
	buttonType: ButtonStories.Base.args?.type,
	buttonTexture: ButtonStories.Base.args?.texture
};
