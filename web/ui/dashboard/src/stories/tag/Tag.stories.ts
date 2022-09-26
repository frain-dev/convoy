import { Story, Meta } from '@storybook/angular/types-6-0';
import { TagComponent } from '../../app/components/tag/tag.component';

export default {
	title: 'Example/Tag',
	component: TagComponent,
	argTypes: {
		type: {
			options: ['grey', 'success', 'warning', 'danger'],
			control: { type: 'select' },
			defaultValue: 'grey'
		},
		class: {
			control: { type: 'text' }
		}
	}
} as Meta;

const Template: Story<TagComponent> = (args: TagComponent) => ({
	props: args,
	template: `<convoy-tag [type]="type">{{ngContent}}</convoy-tag>`
});

export const Base = Template.bind({});
Base.args = {
	ngContent: 'grey convoy tag',
	type: 'grey'
} as Partial<TagComponent>;

export const Success = Template.bind({});
Success.args = {
	ngContent: 'success convoy tag',
	type: 'success'
} as Partial<TagComponent>;

export const Warning = Template.bind({});
Warning.args = {
	ngContent: 'warning convoy tag',
	type: 'warning'
} as Partial<TagComponent>;

export const Danger = Template.bind({});
Danger.args = {
	ngContent: 'danger convoy tag',
	type: 'danger'
} as Partial<TagComponent>;
