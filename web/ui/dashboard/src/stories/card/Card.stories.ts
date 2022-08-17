import { Story, Meta } from '@storybook/angular/types-6-0';
import { CardComponent } from '../../app/components/card/card.component';

export default {
	title: 'Example/Card',
	component: CardComponent,
	argTypes: {
		class: {
			control: { type: 'text' }
		}
	}
} as Meta;

const Template: Story<CardComponent> = (args: CardComponent) => ({
    template: `<convoy-card>{{ngContent}}</convoy-card>`,
	props: args
});

export const Base = Template.bind({});
Base.args = {
	ngContent: 'This is convoy card',
    class: 'p-5'
} as Partial<CardComponent>;
